package orchestrator

import (
	"bytes"
	"fmt"
	"os"
	"runtime"
	"strconv"
	"strings"
	"text/template"
)

// ConditionEvalData represents structured data for condition template evaluation
type ConditionEvalData struct {
	Results map[string]*StageResult `json:"results"`
	Env     map[string]string       `json:"env"`
	OS      map[string]string       `json:"os"`
}

// compareValues compares two values and returns 0 if equal, -1 if a < b, 1 if a > b
func compareValues(a, b any) int {
	aStr := fmt.Sprintf("%v", a)
	bStr := fmt.Sprintf("%v", b)

	if aStr == bStr {
		return 0
	}
	if aStr < bStr {
		return -1
	}
	return 1
}

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
				end = i
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
		"eq": func(a, b any) bool {
			return compareValues(a, b) == 0
		},
		"ne": func(a, b any) bool {
			return compareValues(a, b) != 0
		},
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

	// Snapshot stage results under read lock to avoid race conditions
	o.mu.RLock()
	resultSnapshot := make(map[string]*StageResult, len(o.context.StageResults))
	for k, v := range o.context.StageResults {
		resultSnapshot[k] = v
	}
	o.mu.RUnlock()

	// Create template data with execution context
	data := ConditionEvalData{
		Results: resultSnapshot,
		Env:     o.context.GlobalEnv,
		OS:      map[string]string{"GOOS": runtime.GOOS, "GOARCH": runtime.GOARCH},
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

		o.mu.Lock()
		for _, dependent := range dependents {
			o.context.SkippedStages[dependent] = fmt.Sprintf("dependency %s failed", stage.Name)
		}
		o.mu.Unlock()

		// Continue execution — non-dependent stages will still run
		// Dependents are marked in SkippedStages and will be skipped when reached
		return nil
	default: // "stop" or empty
		return fmt.Errorf("stage %s failed: %w", stage.Name, err)
	}
}

// getFilteredBatches returns execution batches filtered by the given stage range
func (o *Orchestrator) getFilteredBatches(fromStage, toStage string) ([][]string, error) {
	batches, err := o.graph.GetBatches()
	if err != nil {
		return nil, fmt.Errorf("failed to get execution batches: %w", err)
	}

	order, err := o.graph.TopologicalSort()
	if err != nil {
		return nil, fmt.Errorf("failed to sort stages: %w", err)
	}

	stagesToExecute := o.filterStagesInRange(order, fromStage, toStage)
	if len(stagesToExecute) == 0 {
		return nil, nil
	}

	stageSet := make(map[string]bool, len(stagesToExecute))
	for _, stage := range stagesToExecute {
		stageSet[stage] = true
	}

	var filtered [][]string
	for _, batch := range batches {
		var filteredBatch []string
		for _, stage := range batch {
			if stageSet[stage] {
				filteredBatch = append(filteredBatch, stage)
			}
		}
		if len(filteredBatch) > 0 {
			filtered = append(filtered, filteredBatch)
		}
	}

	return filtered, nil
}

// GetExecutionPlan returns the execution plan for stages in the specified range
func (o *Orchestrator) GetExecutionPlan(fromStage, toStage string, direction string) ([][]string, error) {
	if direction == "down" {
		order, err := o.graph.TopologicalSort()
		if err != nil {
			return nil, fmt.Errorf("failed to sort stages: %w", err)
		}

		for i, j := 0, len(order)-1; i < j; i, j = i+1, j-1 {
			order[i], order[j] = order[j], order[i]
		}

		stagesToExecute := o.filterStagesInRangeDown(order, fromStage, toStage)

		batches := make([][]string, len(stagesToExecute))
		for i, stage := range stagesToExecute {
			batches[i] = []string{stage}
		}
		return batches, nil
	}

	return o.getFilteredBatches(fromStage, toStage)
}
