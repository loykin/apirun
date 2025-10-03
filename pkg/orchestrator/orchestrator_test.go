package orchestrator

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
)

func TestOrchestrator_evaluateCondition(t *testing.T) {
	config := &StageOrchestration{
		Global: Global{
			Env: map[string]string{
				"TEST_VAR": "test_value",
			},
		},
	}

	orch := NewOrchestrator(config)

	// Add some test stage results
	orch.context.StageResults["stage1"] = &StageResult{
		Name:    "stage1",
		Success: true,
	}
	orch.context.StageResults["stage2"] = &StageResult{
		Name:    "stage2",
		Success: false,
	}

	tests := []struct {
		name      string
		condition string
		expected  bool
		setupEnv  map[string]string
	}{
		{
			name:      "empty condition should return true",
			condition: "",
			expected:  true,
		},
		{
			name:      "literal true",
			condition: "true",
			expected:  true,
		},
		{
			name:      "literal false",
			condition: "false",
			expected:  false,
		},
		{
			name:      "successful stage check",
			condition: `{{ success "stage1" }}`,
			expected:  true,
		},
		{
			name:      "failed stage check",
			condition: `{{ success "stage2" }}`,
			expected:  false,
		},
		{
			name:      "failed stage check using failed function",
			condition: `{{ failed "stage2" }}`,
			expected:  true,
		},
		{
			name:      "environment variable check",
			condition: `{{ eq (env "TEST_VAR") "test_value" }}`,
			expected:  true,
		},
		{
			name:      "environment variable inequality",
			condition: `{{ ne (env "TEST_VAR") "wrong_value" }}`,
			expected:  true,
		},
		{
			name:      "contains check",
			condition: `{{ contains (env "TEST_VAR") "test" }}`,
			expected:  true,
		},
		{
			name:      "complex condition with success and env",
			condition: `{{ and (success "stage1") (eq (env "TEST_VAR") "test_value") }}`,
			expected:  true,
		},
		{
			name:      "complex condition with OR",
			condition: `{{ or (success "stage2") (success "stage1") }}`,
			expected:  true,
		},
		{
			name:      "nonexistent stage should return false",
			condition: `{{ success "nonexistent" }}`,
			expected:  false,
		},
		{
			name:      "OS environment variable",
			condition: `{{ ne (env "PATH") "" }}`,
			expected:  true,
			setupEnv: map[string]string{
				"PATH": "/usr/bin:/bin",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup environment variables if specified
			var envCleanup []func()
			if tt.setupEnv != nil {
				for k, v := range tt.setupEnv {
					oldVal := os.Getenv(k)
					_ = os.Setenv(k, v)
					// Capture variables in closure to avoid loop variable issues
					envCleanup = append(envCleanup, func(key, val string) func() {
						return func() { _ = os.Setenv(key, val) }
					}(k, oldVal))
				}
			}

			// Cleanup environment variables after test
			defer func() {
				for _, cleanup := range envCleanup {
					cleanup()
				}
			}()

			result := orch.evaluateCondition(tt.condition)
			if result != tt.expected {
				t.Errorf("evaluateCondition(%q) = %v, want %v", tt.condition, result, tt.expected)
			}
		})
	}
}

func TestOrchestrator_evaluateCondition_InvalidTemplate(t *testing.T) {
	config := &StageOrchestration{
		Global: Global{
			Env: map[string]string{},
		},
	}

	orch := NewOrchestrator(config)

	tests := []struct {
		name      string
		condition string
		expected  bool
	}{
		{
			name:      "invalid template syntax",
			condition: `{{ invalid syntax`,
			expected:  false,
		},
		{
			name:      "template that doesn't evaluate to boolean",
			condition: `{{ env "PATH" }}`,
			expected:  false,
		},
		{
			name:      "template with unknown function",
			condition: `{{ unknownFunc "test" }}`,
			expected:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := orch.evaluateCondition(tt.condition)
			if result != tt.expected {
				t.Errorf("evaluateCondition(%q) = %v, want %v", tt.condition, result, tt.expected)
			}
		})
	}
}

func TestOrchestrator_isStageSuccessful(t *testing.T) {
	config := &StageOrchestration{
		Global: Global{},
	}

	orch := NewOrchestrator(config)

	// Add test stage results
	orch.context.StageResults["successful_stage"] = &StageResult{
		Name:    "successful_stage",
		Success: true,
	}
	orch.context.StageResults["failed_stage"] = &StageResult{
		Name:    "failed_stage",
		Success: false,
	}

	tests := []struct {
		name      string
		stageName string
		expected  bool
	}{
		{
			name:      "successful stage",
			stageName: "successful_stage",
			expected:  true,
		},
		{
			name:      "failed stage",
			stageName: "failed_stage",
			expected:  false,
		},
		{
			name:      "nonexistent stage",
			stageName: "nonexistent",
			expected:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := orch.isStageSuccessful(tt.stageName)
			if result != tt.expected {
				t.Errorf("isStageSuccessful(%q) = %v, want %v", tt.stageName, result, tt.expected)
			}
		})
	}
}

func TestOrchestrator_isStageFailed(t *testing.T) {
	config := &StageOrchestration{
		Global: Global{},
	}

	orch := NewOrchestrator(config)

	// Add test stage results
	orch.context.StageResults["successful_stage"] = &StageResult{
		Name:    "successful_stage",
		Success: true,
	}
	orch.context.StageResults["failed_stage"] = &StageResult{
		Name:    "failed_stage",
		Success: false,
	}

	tests := []struct {
		name      string
		stageName string
		expected  bool
	}{
		{
			name:      "successful stage should not be failed",
			stageName: "successful_stage",
			expected:  false,
		},
		{
			name:      "failed stage should be failed",
			stageName: "failed_stage",
			expected:  true,
		},
		{
			name:      "nonexistent stage should not be failed",
			stageName: "nonexistent",
			expected:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := orch.isStageFailed(tt.stageName)
			if result != tt.expected {
				t.Errorf("isStageFailed(%q) = %v, want %v", tt.stageName, result, tt.expected)
			}
		})
	}
}

func TestOrchestrator_handleStageFailure(t *testing.T) {
	tests := []struct {
		name                 string
		stages               []Stage
		failedStage          string
		onFailure            string
		expectedSkippedCount int
		expectedSkipped      []string
		expectError          bool
	}{
		{
			name: "continue on failure",
			stages: []Stage{
				{Name: "stage1", OnFailure: "continue"},
				{Name: "stage2", DependsOn: []string{"stage1"}},
			},
			failedStage:          "stage1",
			onFailure:            "continue",
			expectedSkippedCount: 0,
			expectedSkipped:      []string{},
			expectError:          false,
		},
		{
			name: "stop on failure (default)",
			stages: []Stage{
				{Name: "stage1", OnFailure: "stop"},
				{Name: "stage2", DependsOn: []string{"stage1"}},
			},
			failedStage:          "stage1",
			onFailure:            "stop",
			expectedSkippedCount: 0,
			expectedSkipped:      []string{},
			expectError:          true,
		},
		{
			name: "skip dependents on failure",
			stages: []Stage{
				{Name: "stage1", OnFailure: "skip_dependents"},
				{Name: "stage2", DependsOn: []string{"stage1"}},
				{Name: "stage3", DependsOn: []string{"stage1"}},
				{Name: "stage4", DependsOn: []string{"stage2"}},
			},
			failedStage:          "stage1",
			onFailure:            "skip_dependents",
			expectedSkippedCount: 3,
			expectedSkipped:      []string{"stage2", "stage3", "stage4"},
			expectError:          true,
		},
		{
			name: "skip dependents - no dependents",
			stages: []Stage{
				{Name: "stage1", OnFailure: "skip_dependents"},
				{Name: "stage2"},
			},
			failedStage:          "stage1",
			onFailure:            "skip_dependents",
			expectedSkippedCount: 0,
			expectedSkipped:      []string{},
			expectError:          true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &StageOrchestration{
				Stages: tt.stages,
				Global: Global{},
			}

			orch := NewOrchestrator(config)

			// Build dependency graph
			err := orch.graph.BuildGraph(tt.stages)
			if err != nil {
				t.Fatalf("Failed to build dependency graph: %v", err)
			}

			// Find the failed stage
			var failedStage *Stage
			for i := range tt.stages {
				if tt.stages[i].Name == tt.failedStage {
					failedStage = &tt.stages[i]
					break
				}
			}

			if failedStage == nil {
				t.Fatalf("Failed stage %s not found", tt.failedStage)
			}

			// Simulate stage failure
			testError := fmt.Errorf("test error")
			err = orch.handleStageFailure(failedStage, testError)

			// Check if error expectation is met
			if tt.expectError && err == nil {
				t.Error("Expected error but got none")
			} else if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			// Check skipped stages count
			if len(orch.context.SkippedStages) != tt.expectedSkippedCount {
				t.Errorf("Expected %d skipped stages, got %d",
					tt.expectedSkippedCount, len(orch.context.SkippedStages))
			}

			// Check specific skipped stages
			for _, expectedSkipped := range tt.expectedSkipped {
				if reason, exists := orch.context.SkippedStages[expectedSkipped]; !exists {
					t.Errorf("Expected stage %s to be skipped, but it wasn't", expectedSkipped)
				} else if reason == "" {
					t.Errorf("Expected skip reason for stage %s, but got empty string", expectedSkipped)
				}
			}
		})
	}
}

func TestOrchestrator_executeStage_SkippedStage(t *testing.T) {
	config := &StageOrchestration{
		Global: Global{},
	}

	orch := NewOrchestrator(config)

	// Mark a stage as skipped
	orch.context.SkippedStages["test_stage"] = "dependency failed"

	stage := &Stage{
		Name: "test_stage",
	}

	// Execute the skipped stage
	ctx := context.Background()
	err := orch.executeStage(ctx, stage)

	// Should not return error for skipped stage
	if err != nil {
		t.Errorf("executeStage() on skipped stage returned error: %v", err)
	}

	// Check that result was recorded
	result, exists := orch.context.StageResults["test_stage"]
	if !exists {
		t.Error("Expected stage result to be recorded for skipped stage")
	} else {
		if result.Success {
			t.Error("Expected skipped stage result to have Success = false")
		}
		if result.Error == "" {
			t.Error("Expected skipped stage result to have Error message")
		}
		if !strings.Contains(result.Error, "skipped") {
			t.Errorf("Expected error message to contain 'skipped', got: %s", result.Error)
		}
	}
}

func TestOrchestrator_GetExecutionPlan(t *testing.T) {
	tests := []struct {
		name      string
		stages    []Stage
		fromStage string
		toStage   string
		direction string
		validate  func([][]string) bool
	}{
		{
			name: "up direction - simple linear dependency",
			stages: []Stage{
				{Name: "stage1"},
				{Name: "stage2", DependsOn: []string{"stage1"}},
				{Name: "stage3", DependsOn: []string{"stage2"}},
			},
			fromStage: "",
			toStage:   "",
			direction: "up",
			validate: func(batches [][]string) bool {
				// Should have 3 batches, one stage each
				if len(batches) != 3 {
					return false
				}
				// Check order: stage1, stage2, stage3
				return len(batches[0]) == 1 && batches[0][0] == "stage1" &&
					len(batches[1]) == 1 && batches[1][0] == "stage2" &&
					len(batches[2]) == 1 && batches[2][0] == "stage3"
			},
		},
		{
			name: "up direction - parallel dependencies",
			stages: []Stage{
				{Name: "stage1"},
				{Name: "stage2", DependsOn: []string{"stage1"}},
				{Name: "stage3", DependsOn: []string{"stage1"}},
				{Name: "stage4", DependsOn: []string{"stage2", "stage3"}},
			},
			fromStage: "",
			toStage:   "",
			direction: "up",
			validate: func(batches [][]string) bool {
				// Should have 3 batches
				if len(batches) != 3 {
					return false
				}
				// First batch: stage1
				// Second batch: stage2, stage3 (parallel)
				// Third batch: stage4
				return len(batches[0]) == 1 && batches[0][0] == "stage1" &&
					len(batches[1]) == 2 &&
					len(batches[2]) == 1 && batches[2][0] == "stage4"
			},
		},
		{
			name: "down direction - reverse order",
			stages: []Stage{
				{Name: "stage1"},
				{Name: "stage2", DependsOn: []string{"stage1"}},
				{Name: "stage3", DependsOn: []string{"stage2"}},
			},
			fromStage: "",
			toStage:   "",
			direction: "down",
			validate: func(batches [][]string) bool {
				// Should have 3 batches, reverse order
				if len(batches) != 3 {
					return false
				}
				// Check reverse order: stage3, stage2, stage1
				return len(batches[0]) == 1 && batches[0][0] == "stage3" &&
					len(batches[1]) == 1 && batches[1][0] == "stage2" &&
					len(batches[2]) == 1 && batches[2][0] == "stage1"
			},
		},
		{
			name: "up direction - from/to range",
			stages: []Stage{
				{Name: "stage1"},
				{Name: "stage2", DependsOn: []string{"stage1"}},
				{Name: "stage3", DependsOn: []string{"stage2"}},
				{Name: "stage4", DependsOn: []string{"stage3"}},
			},
			fromStage: "stage2",
			toStage:   "stage3",
			direction: "up",
			validate: func(batches [][]string) bool {
				// Should only include stage2 and stage3
				if len(batches) != 2 {
					return false
				}
				return len(batches[0]) == 1 && batches[0][0] == "stage2" &&
					len(batches[1]) == 1 && batches[1][0] == "stage3"
			},
		},
		{
			name: "empty result - no stages in range",
			stages: []Stage{
				{Name: "stage1"},
				{Name: "stage2", DependsOn: []string{"stage1"}},
			},
			fromStage: "nonexistent",
			toStage:   "nonexistent2",
			direction: "up",
			validate: func(batches [][]string) bool {
				return len(batches) == 0
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &StageOrchestration{
				Stages: tt.stages,
				Global: Global{},
			}

			orch := NewOrchestrator(config)

			// Build dependency graph
			err := orch.graph.BuildGraph(tt.stages)
			if err != nil {
				t.Fatalf("Failed to build dependency graph: %v", err)
			}

			// Get execution plan
			batches, err := orch.GetExecutionPlan(tt.fromStage, tt.toStage, tt.direction)
			if err != nil {
				t.Fatalf("GetExecutionPlan() failed: %v", err)
			}

			// Validate result
			if !tt.validate(batches) {
				t.Errorf("Execution plan validation failed for batches: %v", batches)
			}
		})
	}
}

func TestOrchestrator_GetExecutionPlan_CircularDependency(t *testing.T) {
	stages := []Stage{
		{Name: "stage1", DependsOn: []string{"stage2"}},
		{Name: "stage2", DependsOn: []string{"stage1"}},
	}

	config := &StageOrchestration{
		Stages: stages,
		Global: Global{},
	}

	orch := NewOrchestrator(config)

	// Build dependency graph (this should succeed)
	err := orch.graph.BuildGraph(stages)
	if err != nil {
		t.Fatalf("Failed to build dependency graph: %v", err)
	}

	// GetExecutionPlan should fail due to circular dependency
	_, err = orch.GetExecutionPlan("", "", "up")
	if err == nil {
		t.Error("Expected error for circular dependency, but got none")
	}
}
