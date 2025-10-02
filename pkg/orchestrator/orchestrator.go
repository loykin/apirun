package orchestrator

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"text/template"
	"time"

	"github.com/loykin/apirun"
	"github.com/loykin/apirun/pkg/env"
	"gopkg.in/yaml.v3"
)

// Orchestrator manages the execution of multiple stages
type Orchestrator struct {
	config  *StageOrchestration
	graph   *DependencyGraph
	context *ExecutionContext
	logger  *slog.Logger
	mu      sync.RWMutex
}

// NewOrchestrator creates a new orchestrator with the given configuration
func NewOrchestrator(config *StageOrchestration) *Orchestrator {
	return &Orchestrator{
		config: config,
		graph:  NewDependencyGraph(),
		context: &ExecutionContext{
			StageResults:  make(map[string]*StageResult),
			GlobalEnv:     config.Global.Env,
			SkippedStages: make(map[string]string),
		},
		logger: slog.With("component", "orchestrator"),
	}
}

// LoadFromFile creates an orchestrator from a configuration file
func LoadFromFile(configPath string) (*Orchestrator, error) {
	config, err := LoadStageOrchestration(configPath)
	if err != nil {
		return nil, err
	}

	orchestrator := NewOrchestrator(config)
	if err := orchestrator.initialize(); err != nil {
		return nil, fmt.Errorf("failed to initialize orchestrator: %w", err)
	}

	return orchestrator, nil
}

// initialize sets up the orchestrator
func (o *Orchestrator) initialize() error {
	// Build dependency graph
	if err := o.graph.BuildGraph(o.config.Stages); err != nil {
		return fmt.Errorf("failed to build dependency graph: %w", err)
	}

	o.logger.Info("orchestrator initialized",
		"stages_count", len(o.config.Stages),
		"global_env_vars", len(o.config.Global.Env))

	return nil
}

// ExecuteStages executes stages based on the specified range
func (o *Orchestrator) ExecuteStages(ctx context.Context, fromStage, toStage string) error {
	o.logger.Info("starting stage execution",
		"from_stage", fromStage,
		"to_stage", toStage)

	// Get execution order
	order, err := o.graph.TopologicalSort()
	if err != nil {
		return fmt.Errorf("failed to determine execution order: %w", err)
	}

	// Filter stages based on from/to range
	filteredOrder := o.filterStagesInRange(order, fromStage, toStage)
	if len(filteredOrder) == 0 {
		o.logger.Info("no stages to execute in the specified range")
		return nil
	}

	o.logger.Info("execution plan determined",
		"total_stages", len(filteredOrder),
		"stages", filteredOrder)

	// Execute stages in order
	for i, stageName := range filteredOrder {
		stage := o.getStageByName(stageName)
		if stage == nil {
			return fmt.Errorf("stage not found: %s", stageName)
		}

		o.logger.Info("executing stage",
			"stage", stageName,
			"progress", fmt.Sprintf("%d/%d", i+1, len(filteredOrder)))

		if err := o.executeStage(ctx, stage); err != nil {
			return o.handleStageFailure(stage, err)
		}

		// Wait between stages if configured
		if i < len(filteredOrder)-1 && o.config.Global.WaitBetweenStages > 0 {
			o.logger.Debug("waiting between stages",
				"duration", o.config.Global.WaitBetweenStages)
			time.Sleep(o.config.Global.WaitBetweenStages)
		}
	}

	o.logger.Info("stage execution completed successfully",
		"executed_stages", len(filteredOrder))

	return nil
}

// ExecuteStagesDown executes down migrations for stages in reverse order
func (o *Orchestrator) ExecuteStagesDown(ctx context.Context, fromStage, toStage string) error {
	o.logger.Info("starting stage down execution",
		"from_stage", fromStage,
		"to_stage", toStage)

	// Get execution order and reverse it
	order, err := o.graph.TopologicalSort()
	if err != nil {
		return fmt.Errorf("failed to determine execution order: %w", err)
	}

	// Reverse the order for down migrations
	for i, j := 0, len(order)-1; i < j; i, j = i+1, j-1 {
		order[i], order[j] = order[j], order[i]
	}

	// Filter stages based on from/to range (note: semantics are different for down)
	filteredOrder := o.filterStagesInRangeDown(order, fromStage, toStage)
	if len(filteredOrder) == 0 {
		o.logger.Info("no stages to execute down in the specified range")
		return nil
	}

	o.logger.Info("down execution plan determined",
		"total_stages", len(filteredOrder),
		"stages", filteredOrder)

	// Execute down migrations
	for i, stageName := range filteredOrder {
		stage := o.getStageByName(stageName)
		if stage == nil {
			return fmt.Errorf("stage not found: %s", stageName)
		}

		o.logger.Info("executing stage down",
			"stage", stageName,
			"progress", fmt.Sprintf("%d/%d", i+1, len(filteredOrder)))

		if err := o.executeStageDown(ctx, stage); err != nil {
			return o.handleStageFailure(stage, err)
		}

		// Wait between stages if configured
		if i < len(filteredOrder)-1 && o.config.Global.WaitBetweenStages > 0 {
			time.Sleep(o.config.Global.WaitBetweenStages)
		}
	}

	o.logger.Info("stage down execution completed successfully",
		"executed_stages", len(filteredOrder))

	return nil
}

// executeStage executes a single stage
func (o *Orchestrator) executeStage(ctx context.Context, stage *Stage) error {
	// Check if stage is marked as skipped
	if reason, isSkipped := o.context.SkippedStages[stage.Name]; isSkipped {
		o.logger.Info("stage skipped due to dependency failure",
			"stage", stage.Name,
			"reason", reason)

		result := &StageResult{
			Name:         stage.Name,
			Success:      false,
			Error:        fmt.Sprintf("skipped: %s", reason),
			StartTime:    time.Now(),
			EndTime:      time.Now(),
			Duration:     0,
			ExtractedEnv: make(map[string]string),
		}

		o.mu.Lock()
		o.context.StageResults[stage.Name] = result
		o.mu.Unlock()

		return nil // Don't propagate error for skipped stages
	}

	result := &StageResult{
		Name:         stage.Name,
		ExtractedEnv: make(map[string]string),
		StartTime:    time.Now(),
	}

	defer func() {
		result.EndTime = time.Now()
		result.Duration = result.EndTime.Sub(result.StartTime)
		o.mu.Lock()
		o.context.StageResults[stage.Name] = result
		o.mu.Unlock()
	}()

	// Check condition if specified
	if stage.Condition != "" && !o.evaluateCondition(stage.Condition) {
		o.logger.Info("stage skipped due to condition",
			"stage", stage.Name,
			"condition", stage.Condition)
		result.Success = true
		return nil
	}

	// Build environment for this stage
	stageEnv, err := o.buildStageEnvironment(stage)
	if err != nil {
		result.Error = err.Error()
		return fmt.Errorf("failed to build environment for stage %s: %w", stage.Name, err)
	}

	// Load stage configuration
	config, err := o.loadStageConfig(stage.ConfigPath)
	if err != nil {
		result.Error = err.Error()
		return fmt.Errorf("failed to load config for stage %s: %w", stage.Name, err)
	}

	// Execute the stage using apirun Migrator
	migrator := &apirun.Migrator{
		Dir:         config.MigrateDir,
		Env:         stageEnv,
		Auth:        config.Auth,
		StoreConfig: config.StoreConfig,
	}

	// Apply stage timeout if specified
	stageCtx := ctx
	if stage.Timeout > 0 {
		var cancel context.CancelFunc
		stageCtx, cancel = context.WithTimeout(ctx, stage.Timeout)
		defer cancel()
	}

	execResults, err := migrator.MigrateUp(stageCtx, 0)
	if err != nil {
		result.Error = err.Error()
		return err
	}

	// Collect extracted environment variables from all migration results
	for _, execResult := range execResults {
		if execResult.Result != nil {
			for k, v := range execResult.Result.ExtractedEnv {
				result.ExtractedEnv[k] = v
			}
		}
	}

	result.Success = true
	o.logger.Info("stage executed successfully",
		"stage", stage.Name,
		"duration", result.Duration,
		"extracted_vars", len(result.ExtractedEnv))

	return nil
}

// executeStageDown executes a single stage's down migration
func (o *Orchestrator) executeStageDown(ctx context.Context, stage *Stage) error {
	// Build environment for this stage (simplified for down migrations)
	stageEnv, err := o.buildStageEnvironmentForDown(stage)
	if err != nil {
		return fmt.Errorf("failed to build environment for stage %s: %w", stage.Name, err)
	}

	// Load stage configuration
	config, err := o.loadStageConfig(stage.ConfigPath)
	if err != nil {
		return fmt.Errorf("failed to load config for stage %s: %w", stage.Name, err)
	}

	// Execute down migration
	migrator := &apirun.Migrator{
		Dir:         config.MigrateDir,
		Env:         stageEnv,
		Auth:        config.Auth,
		StoreConfig: config.StoreConfig,
	}

	// Apply stage timeout if specified
	stageCtx := ctx
	if stage.Timeout > 0 {
		var cancel context.CancelFunc
		stageCtx, cancel = context.WithTimeout(ctx, stage.Timeout)
		defer cancel()
	}

	_, err = migrator.MigrateDown(stageCtx, 0)
	if err != nil {
		return err
	}

	o.logger.Info("stage down executed successfully", "stage", stage.Name)
	return nil
}

// buildStageEnvironment builds the environment for a stage
func (o *Orchestrator) buildStageEnvironment(stage *Stage) (*env.Env, error) {
	stageEnv := env.New()

	// Start with global environment
	if o.context.GlobalEnv != nil {
		for k, v := range o.context.GlobalEnv {
			_ = stageEnv.SetString("global", k, v)
		}
	}

	// Add stage-specific environment
	if stage.Env != nil {
		for k, v := range stage.Env {
			_ = stageEnv.SetString("local", k, v)
		}
	}

	// Add environment from dependent stages
	o.mu.RLock()
	for _, envFromStage := range stage.EnvFromStages {
		stageResult, exists := o.context.StageResults[envFromStage.Stage]
		if !exists {
			o.mu.RUnlock()
			return nil, fmt.Errorf("dependent stage %s has not been executed", envFromStage.Stage)
		}

		for _, varName := range envFromStage.Vars {
			if value, exists := stageResult.ExtractedEnv[varName]; exists {
				_ = stageEnv.SetString("local", varName, value)
			} else {
				o.logger.Warn("variable not found in dependent stage",
					"stage", stage.Name,
					"dependent_stage", envFromStage.Stage,
					"variable", varName)
			}
		}
	}
	o.mu.RUnlock()

	return stageEnv, nil
}

// buildStageEnvironmentForDown builds a simplified environment for down migrations
// that doesn't depend on other stages since they may have already been rolled back
func (o *Orchestrator) buildStageEnvironmentForDown(stage *Stage) (*env.Env, error) {
	stageEnv := env.New()

	// Start with global environment
	if o.context.GlobalEnv != nil {
		for k, v := range o.context.GlobalEnv {
			_ = stageEnv.SetString("global", k, v)
		}
	}

	// Add stage-specific environment (not dependent stage variables)
	if stage.Env != nil {
		for k, v := range stage.Env {
			_ = stageEnv.SetString("local", k, v)
		}
	}

	// For down migrations, we don't add environment from dependent stages
	// since they may have already been rolled back

	return stageEnv, nil
}

// loadStageConfig loads the configuration for a stage
func (o *Orchestrator) loadStageConfig(configPath string) (*StageConfig, error) {
	// #nosec G304 -- path is validated during orchestration loading
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config StageConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	// Resolve relative paths
	baseDir := filepath.Dir(configPath)
	if config.MigrateDir != "" && !filepath.IsAbs(config.MigrateDir) {
		config.MigrateDir = filepath.Join(baseDir, config.MigrateDir)
	}

	return &config, nil
}

// StageConfig represents the configuration for a single stage
type StageConfig struct {
	MigrateDir  string              `yaml:"migrate_dir"`
	Auth        []apirun.Auth       `yaml:"auth"`
	Env         map[string]string   `yaml:"env"`
	StoreConfig *apirun.StoreConfig `yaml:"store"`
}

// Helper methods

func (o *Orchestrator) getStageByName(name string) *Stage {
	for i := range o.config.Stages {
		if o.config.Stages[i].Name == name {
			return &o.config.Stages[i]
		}
	}
	return nil
}

func (o *Orchestrator) filterStagesInRange(order []string, fromStage, toStage string) []string {
	if fromStage == "" && toStage == "" {
		return order
	}

	start, end := 0, len(order)
	fromFound, toFound := fromStage == "", toStage == ""

	if fromStage != "" {
		for i, stage := range order {
			if stage == fromStage {
				start = i
				fromFound = true
				break
			}
		}
	}

	if toStage != "" {
		for i, stage := range order {
			if stage == toStage {
				end = i + 1
				toFound = true
				break
			}
		}
	}

	// If any specified stage is not found, return empty slice
	if !fromFound || !toFound {
		return []string{}
	}

	if start >= end {
		return []string{}
	}

	return order[start:end]
}

func (o *Orchestrator) filterStagesInRangeDown(order []string, fromStage, toStage string) []string {
	// For down migrations, the semantics are different
	// fromStage is where to start rolling back from (inclusive)
	// toStage is where to stop rolling back (exclusive)
	if fromStage == "" && toStage == "" {
		return order
	}

	start, end := 0, len(order)

	if fromStage != "" {
		for i, stage := range order {
			if stage == fromStage {
				start = i
				break
			}
		}
	}

	if toStage != "" {
		for i, stage := range order {
			if stage == toStage {
				end = i
				break
			}
		}
	}

	if start >= end {
		return []string{}
	}

	return order[start:end]
}

func (o *Orchestrator) evaluateCondition(condition string) bool {
	condition = strings.TrimSpace(condition)
	if condition == "" {
		return true
	}
	if condition == "true" {
		return true
	}
	if condition == "false" {
		return false
	}

	// Template-based condition evaluation
	tmpl, err := template.New("condition").Funcs(template.FuncMap{
		"eq":       func(a, b interface{}) bool { return fmt.Sprintf("%v", a) == fmt.Sprintf("%v", b) },
		"ne":       func(a, b interface{}) bool { return fmt.Sprintf("%v", a) != fmt.Sprintf("%v", b) },
		"contains": func(s, substr string) bool { return strings.Contains(s, substr) },
		"success":  func(stageName string) bool { return o.isStageSuccessful(stageName) },
		"failed":   func(stageName string) bool { return o.isStageFailed(stageName) },
		"env": func(key string) string {
			// Check Global.Env first, then OS env
			if val, exists := o.context.GlobalEnv[key]; exists {
				return val
			}
			return os.Getenv(key)
		},
	}).Parse(condition)

	if err != nil {
		o.logger.Error("failed to parse condition template", "condition", condition, "error", err)
		return false
	}

	// Create template data with execution context
	data := map[string]interface{}{
		"Results": o.context.StageResults,
		"Env":     o.context.GlobalEnv,
		"OS":      map[string]string{"GOOS": "darwin", "GOARCH": "amd64"}, // Could be dynamic
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		o.logger.Error("failed to execute condition template", "condition", condition, "error", err)
		return false
	}

	result := strings.TrimSpace(buf.String())
	parsed, err := strconv.ParseBool(result)
	if err != nil {
		o.logger.Error("condition did not evaluate to boolean", "condition", condition, "result", result)
		return false
	}

	o.logger.Debug("condition evaluated", "condition", condition, "result", parsed)
	return parsed
}

func (o *Orchestrator) isStageSuccessful(stageName string) bool {
	if result, exists := o.context.StageResults[stageName]; exists {
		return result.Success
	}
	return false
}

func (o *Orchestrator) isStageFailed(stageName string) bool {
	if result, exists := o.context.StageResults[stageName]; exists {
		return !result.Success
	}
	return false
}

func (o *Orchestrator) handleStageFailure(stage *Stage, err error) error {
	o.logger.Error("stage execution failed",
		"stage", stage.Name,
		"error", err,
		"on_failure", stage.OnFailure)

	switch stage.OnFailure {
	case "continue":
		o.logger.Warn("continuing despite stage failure", "stage", stage.Name)
		return nil
	case "skip_dependents":
		dependents := o.graph.GetAllDependents(stage.Name)
		o.logger.Warn("skipping dependent stages due to failure",
			"failed_stage", stage.Name,
			"dependents", dependents)

		// Mark all dependents as skipped
		for _, dependent := range dependents {
			o.context.SkippedStages[dependent] = fmt.Sprintf("dependency %s failed", stage.Name)
		}

		return err
	default: // "stop" or empty
		return fmt.Errorf("stage %s failed: %w", stage.Name, err)
	}
}

// GetStageResults returns the results of executed stages
func (o *Orchestrator) GetStageResults() map[string]*StageResult {
	o.mu.RLock()
	defer o.mu.RUnlock()

	results := make(map[string]*StageResult)
	for k, v := range o.context.StageResults {
		results[k] = v
	}
	return results
}

// GetExecutionPlan returns the execution plan for stages in the specified range
func (o *Orchestrator) GetExecutionPlan(fromStage, toStage string, direction string) ([][]string, error) {
	if direction == "down" {
		// For down migrations, use reverse order
		order, err := o.graph.TopologicalSort()
		if err != nil {
			return nil, fmt.Errorf("failed to sort stages: %w", err)
		}

		// Reverse the order for down migrations
		for i, j := 0, len(order)-1; i < j; i, j = i+1, j-1 {
			order[i], order[j] = order[j], order[i]
		}

		stagesToExecute := o.filterStagesInRange(order, fromStage, toStage)

		// Convert to batches (each stage in its own batch for down migrations)
		batches := make([][]string, len(stagesToExecute))
		for i, stage := range stagesToExecute {
			batches[i] = []string{stage}
		}
		return batches, nil
	}

	// For up migrations, use dependency batches
	batches, err := o.graph.GetBatches()
	if err != nil {
		return nil, fmt.Errorf("failed to get execution batches: %w", err)
	}

	// Filter batches based on range
	var filteredBatches [][]string
	stageSet := make(map[string]bool)

	// Build set of stages to execute
	order, err := o.graph.TopologicalSort()
	if err != nil {
		return nil, fmt.Errorf("failed to sort stages: %w", err)
	}

	stagesToExecute := o.filterStagesInRange(order, fromStage, toStage)
	for _, stage := range stagesToExecute {
		stageSet[stage] = true
	}

	// Filter each batch to only include stages in our execution set
	for _, batch := range batches {
		var filteredBatch []string
		for _, stage := range batch {
			if stageSet[stage] {
				filteredBatch = append(filteredBatch, stage)
			}
		}
		if len(filteredBatch) > 0 {
			filteredBatches = append(filteredBatches, filteredBatch)
		}
	}

	return filteredBatches, nil
}
