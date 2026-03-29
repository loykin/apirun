package orchestrator

import (
	"context"
	"testing"
	"time"
)

func TestContextSleep(t *testing.T) {
	tests := []struct {
		name     string
		duration time.Duration
		timeout  time.Duration
		wantErr  bool
	}{
		{
			name:     "sleep completes normally",
			duration: 10 * time.Millisecond,
			timeout:  100 * time.Millisecond,
			wantErr:  false,
		},
		{
			name:     "context cancelled before sleep completes",
			duration: 100 * time.Millisecond,
			timeout:  10 * time.Millisecond,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), tt.timeout)
			defer cancel()

			start := time.Now()
			err := contextSleep(ctx, tt.duration)
			elapsed := time.Since(start)

			if tt.wantErr && err == nil {
				t.Errorf("contextSleep() expected error but got nil")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("contextSleep() unexpected error: %v", err)
			}

			// Verify timing behavior
			if tt.wantErr {
				// Should return quickly when context is cancelled
				if elapsed > tt.timeout*2 {
					t.Errorf("contextSleep() took too long when cancelled: %v", elapsed)
				}
			} else {
				// Should sleep for the full duration
				if elapsed < tt.duration {
					t.Errorf("contextSleep() returned too early: %v < %v", elapsed, tt.duration)
				}
			}
		})
	}
}

func TestNewOrchestrator(t *testing.T) {
	config := &StageOrchestration{
		Global: Global{
			Env: map[string]string{
				"TEST_VAR": "test_value",
			},
		},
		Stages: []Stage{
			{Name: "stage1"},
			{Name: "stage2"},
		},
	}

	orch := NewOrchestrator(config)

	if orch == nil {
		t.Fatal("NewOrchestrator() returned nil")
	}

	if orch.config != config {
		t.Error("NewOrchestrator() config not set correctly")
	}

	if orch.graph == nil {
		t.Error("NewOrchestrator() dependency graph not initialized")
	}

	if orch.context == nil {
		t.Error("NewOrchestrator() execution context not initialized")
	}

	if orch.logger == nil {
		t.Error("NewOrchestrator() logger not initialized")
	}

	// Check that execution context is properly initialized
	if orch.context.StageResults == nil {
		t.Error("NewOrchestrator() stage results not initialized")
	}

	if orch.context.SkippedStages == nil {
		t.Error("NewOrchestrator() skipped stages not initialized")
	}

	if len(orch.context.GlobalEnv) != 1 || orch.context.GlobalEnv["TEST_VAR"] != "test_value" {
		t.Error("NewOrchestrator() global env not set correctly")
	}
}

func TestOrchestrator_initialize(t *testing.T) {
	tests := []struct {
		name    string
		config  *StageOrchestration
		wantErr bool
	}{
		{
			name: "successful initialization",
			config: &StageOrchestration{
				Stages: []Stage{
					{Name: "stage1"},
					{Name: "stage2", DependsOn: []string{"stage1"}},
				},
			},
			wantErr: false,
		},
		{
			name: "circular dependency builds successfully but would fail in execution",
			config: &StageOrchestration{
				Stages: []Stage{
					{Name: "stage1", DependsOn: []string{"stage2"}},
					{Name: "stage2", DependsOn: []string{"stage1"}},
				},
			},
			wantErr: false, // BuildGraph doesn't validate cycles, only topological sort does
		},
		{
			name: "missing dependency error",
			config: &StageOrchestration{
				Stages: []Stage{
					{Name: "stage1", DependsOn: []string{"nonexistent"}},
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			orch := NewOrchestrator(tt.config)
			err := orch.initialize()

			if tt.wantErr && err == nil {
				t.Errorf("initialize() expected error but got nil")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("initialize() unexpected error: %v", err)
			}
		})
	}
}

func TestOrchestrator_GetStageResults(t *testing.T) {
	orch := NewOrchestrator(&StageOrchestration{})

	// Add some test results
	result1 := &StageResult{Name: "stage1", Success: true}
	result2 := &StageResult{Name: "stage2", Success: false}

	orch.context.StageResults["stage1"] = result1
	orch.context.StageResults["stage2"] = result2

	results := orch.GetStageResults()

	if len(results) != 2 {
		t.Errorf("GetStageResults() expected 2 results, got %d", len(results))
	}

	if results["stage1"] == nil || results["stage1"].Name != result1.Name || results["stage1"].Success != result1.Success {
		t.Error("GetStageResults() stage1 result not correct")
	}

	if results["stage2"] == nil || results["stage2"].Name != result2.Name || results["stage2"].Success != result2.Success {
		t.Error("GetStageResults() stage2 result not correct")
	}

	// Verify it returns a copy (modifications shouldn't affect original)
	delete(results, "stage1")
	if len(orch.context.StageResults) != 2 {
		t.Error("GetStageResults() should return a copy, but modifications affected original")
	}
}

func TestLoadFromFile_FileNotFound(t *testing.T) {
	_, err := LoadFromFile("nonexistent.yaml")
	if err == nil {
		t.Error("LoadFromFile() expected error for nonexistent file")
	}
}
