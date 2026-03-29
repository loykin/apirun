package orchestrator

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/loykin/apirun"
	"github.com/loykin/apirun/pkg/env"
)

// ExecuteStages executes stages in dependency order, running independent stages concurrently within each batch
func (o *Orchestrator) ExecuteStages(ctx context.Context, fromStage, toStage string) error {
	o.logger.Info("starting stage execution",
		"from_stage", fromStage,
		"to_stage", toStage)

	filteredBatches, err := o.getFilteredBatches(fromStage, toStage)
	if err != nil {
		return fmt.Errorf("failed to determine execution order: %w", err)
	}
	if len(filteredBatches) == 0 {
		o.logger.Info("no stages to execute in the specified range")
		return nil
	}

	totalStages := 0
	for _, b := range filteredBatches {
		totalStages += len(b)
	}
	o.logger.Info("execution plan determined",
		"total_stages", totalStages,
		"batches", len(filteredBatches))

	for i, batch := range filteredBatches {
		o.logger.Info("executing batch",
			"batch", fmt.Sprintf("%d/%d", i+1, len(filteredBatches)),
			"stages", batch)

		if err := o.executeBatch(ctx, batch); err != nil {
			return err
		}

		if i < len(filteredBatches)-1 && o.config.Global.WaitBetweenStages > 0 {
			if err := contextSleep(ctx, o.config.Global.WaitBetweenStages); err != nil {
				return fmt.Errorf("interrupted while waiting between batches: %w", err)
			}
		}
	}

	o.logger.Info("stage execution completed successfully", "executed_stages", totalStages)
	return nil
}

// executeBatch runs all stages in the batch concurrently and waits for all to finish
func (o *Orchestrator) executeBatch(ctx context.Context, batch []string) error {
	if len(batch) == 1 {
		stage := o.getStageByName(batch[0])
		if stage == nil {
			return fmt.Errorf("stage not found: %s", batch[0])
		}
		if err := o.executeStage(ctx, stage); err != nil {
			return o.handleStageFailure(stage, err)
		}
		return nil
	}

	var wg sync.WaitGroup
	errCh := make(chan error, len(batch))

	for _, stageName := range batch {
		stage := o.getStageByName(stageName)
		if stage == nil {
			return fmt.Errorf("stage not found: %s", stageName)
		}
		wg.Add(1)
		go func(s *Stage) {
			defer wg.Done()
			if err := o.executeStage(ctx, s); err != nil {
				if handleErr := o.handleStageFailure(s, err); handleErr != nil {
					errCh <- handleErr
				}
			}
		}(stage)
	}

	wg.Wait()
	close(errCh)

	for err := range errCh {
		if err != nil {
			return err
		}
	}
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
			if err := contextSleep(ctx, o.config.Global.WaitBetweenStages); err != nil {
				return fmt.Errorf("interrupted while waiting between stages: %w", err)
			}
		}
	}

	o.logger.Info("stage down execution completed successfully",
		"executed_stages", len(filteredOrder))

	return nil
}

// executeStage executes a single stage
func (o *Orchestrator) executeStage(ctx context.Context, stage *Stage) error {
	// Check if stage is marked as skipped
	o.mu.RLock()
	reason, isSkipped := o.context.SkippedStages[stage.Name]
	o.mu.RUnlock()

	if isSkipped {
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
