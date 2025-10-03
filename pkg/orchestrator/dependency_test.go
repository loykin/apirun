package orchestrator

import (
	"reflect"
	"sort"
	"testing"
)

func TestDependencyGraph_AddStage(t *testing.T) {
	dg := NewDependencyGraph()

	stage := &Stage{
		Name:      "test_stage",
		DependsOn: []string{"dep1", "dep2"},
	}

	dg.AddStage(stage)

	if node, exists := dg.nodes["test_stage"]; !exists {
		t.Error("Stage was not added to dependency graph")
	} else {
		if node.Name != "test_stage" {
			t.Errorf("Expected stage name 'test_stage', got %s", node.Name)
		}
		if node.Stage != stage {
			t.Error("Stage reference is incorrect")
		}
	}
}

func TestDependencyGraph_AddDependency(t *testing.T) {
	dg := NewDependencyGraph()

	// Add stages first
	dg.AddStage(&Stage{Name: "stage1"})
	dg.AddStage(&Stage{Name: "stage2"})

	err := dg.AddDependency("stage1", "stage2")
	if err != nil {
		t.Errorf("Unexpected error adding dependency: %v", err)
	}

	// Check that dependency was added (stage1 -> stage2)
	if _, exists := dg.nodes["stage2"]; exists {
		found := false
		if deps, exists := dg.edges["stage1"]; exists {
			for _, dep := range deps {
				if dep == "stage2" {
					found = true
					break
				}
			}
		}
		if !found {
			t.Error("Dependency was not added correctly")
		}
	} else {
		t.Error("Stage2 not found in graph")
	}
}

func TestDependencyGraph_AddDependency_NonexistentStage(t *testing.T) {
	dg := NewDependencyGraph()

	err := dg.AddDependency("nonexistent1", "nonexistent2")
	if err == nil {
		t.Error("Expected error when adding dependency between nonexistent stages")
	}
}

func TestDependencyGraph_GetDependents(t *testing.T) {
	dg := NewDependencyGraph()

	// Create test stages
	stages := []*Stage{
		{Name: "stage1"},
		{Name: "stage2", DependsOn: []string{"stage1"}},
		{Name: "stage3", DependsOn: []string{"stage1"}},
		{Name: "stage4", DependsOn: []string{"stage2"}},
	}

	// Add stages to graph
	for _, stage := range stages {
		dg.AddStage(stage)
	}

	// Build graph
	err := dg.BuildGraph([]Stage{
		*stages[0], *stages[1], *stages[2], *stages[3],
	})
	if err != nil {
		t.Fatalf("Failed to build graph: %v", err)
	}

	tests := []struct {
		name      string
		stageName string
		expected  []string
	}{
		{
			name:      "stage1 has two direct dependents",
			stageName: "stage1",
			expected:  []string{"stage2", "stage3"},
		},
		{
			name:      "stage2 has one dependent",
			stageName: "stage2",
			expected:  []string{"stage4"},
		},
		{
			name:      "stage3 has no dependents",
			stageName: "stage3",
			expected:  []string{},
		},
		{
			name:      "stage4 has no dependents",
			stageName: "stage4",
			expected:  []string{},
		},
		{
			name:      "nonexistent stage has no dependents",
			stageName: "nonexistent",
			expected:  []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := dg.GetDependents(tt.stageName)
			sort.Strings(result)
			sort.Strings(tt.expected)

			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("GetDependents(%s) = %v, want %v", tt.stageName, result, tt.expected)
			}
		})
	}
}

func TestDependencyGraph_GetAllDependents(t *testing.T) {
	dg := NewDependencyGraph()

	// Create a more complex dependency tree:
	// stage1 -> stage2 -> stage4
	//        -> stage3 -> stage5
	stages := []*Stage{
		{Name: "stage1"},
		{Name: "stage2", DependsOn: []string{"stage1"}},
		{Name: "stage3", DependsOn: []string{"stage1"}},
		{Name: "stage4", DependsOn: []string{"stage2"}},
		{Name: "stage5", DependsOn: []string{"stage3"}},
	}

	// Add stages to graph
	for _, stage := range stages {
		dg.AddStage(stage)
	}

	// Build graph
	err := dg.BuildGraph([]Stage{
		*stages[0], *stages[1], *stages[2], *stages[3], *stages[4],
	})
	if err != nil {
		t.Fatalf("Failed to build graph: %v", err)
	}

	tests := []struct {
		name      string
		stageName string
		expected  []string
	}{
		{
			name:      "stage1 has all dependents recursively",
			stageName: "stage1",
			expected:  []string{"stage2", "stage3", "stage4", "stage5"},
		},
		{
			name:      "stage2 has one recursive dependent",
			stageName: "stage2",
			expected:  []string{"stage4"},
		},
		{
			name:      "stage3 has one recursive dependent",
			stageName: "stage3",
			expected:  []string{"stage5"},
		},
		{
			name:      "stage4 has no dependents",
			stageName: "stage4",
			expected:  []string{},
		},
		{
			name:      "stage5 has no dependents",
			stageName: "stage5",
			expected:  []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := dg.GetAllDependents(tt.stageName)
			sort.Strings(result)
			sort.Strings(tt.expected)

			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("GetAllDependents(%s) = %v, want %v", tt.stageName, result, tt.expected)
			}
		})
	}
}

func TestDependencyGraph_DetectCycle(t *testing.T) {
	tests := []struct {
		name     string
		stages   []Stage
		hasCycle bool
	}{
		{
			name: "no cycle - linear dependency",
			stages: []Stage{
				{Name: "stage1"},
				{Name: "stage2", DependsOn: []string{"stage1"}},
				{Name: "stage3", DependsOn: []string{"stage2"}},
			},
			hasCycle: false,
		},
		{
			name: "no cycle - parallel dependencies",
			stages: []Stage{
				{Name: "stage1"},
				{Name: "stage2", DependsOn: []string{"stage1"}},
				{Name: "stage3", DependsOn: []string{"stage1"}},
			},
			hasCycle: false,
		},
		{
			name: "cycle detected - simple cycle",
			stages: []Stage{
				{Name: "stage1", DependsOn: []string{"stage2"}},
				{Name: "stage2", DependsOn: []string{"stage1"}},
			},
			hasCycle: true,
		},
		{
			name: "cycle detected - indirect cycle",
			stages: []Stage{
				{Name: "stage1", DependsOn: []string{"stage3"}},
				{Name: "stage2", DependsOn: []string{"stage1"}},
				{Name: "stage3", DependsOn: []string{"stage2"}},
			},
			hasCycle: true,
		},
		{
			name: "self-dependency cycle",
			stages: []Stage{
				{Name: "stage1", DependsOn: []string{"stage1"}},
			},
			hasCycle: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dg := NewDependencyGraph()

			err := dg.BuildGraph(tt.stages)
			if err != nil {
				t.Fatalf("Failed to build graph: %v", err)
			}

			cycle := dg.DetectCycle()
			hasCycle := len(cycle) > 0

			if hasCycle != tt.hasCycle {
				t.Errorf("DetectCycle() hasCycle = %v, want %v", hasCycle, tt.hasCycle)
				if hasCycle {
					t.Errorf("Detected cycle: %v", cycle)
				}
			}
		})
	}
}

func TestDependencyGraph_TopologicalSort(t *testing.T) {
	tests := []struct {
		name     string
		stages   []Stage
		hasError bool
		validate func([]string) bool
	}{
		{
			name: "simple linear dependency",
			stages: []Stage{
				{Name: "stage1"},
				{Name: "stage2", DependsOn: []string{"stage1"}},
				{Name: "stage3", DependsOn: []string{"stage2"}},
			},
			hasError: false,
			validate: func(order []string) bool {
				// stage1 should come before stage2, stage2 before stage3
				stage1Idx, stage2Idx, stage3Idx := -1, -1, -1
				for i, stage := range order {
					switch stage {
					case "stage1":
						stage1Idx = i
					case "stage2":
						stage2Idx = i
					case "stage3":
						stage3Idx = i
					}
				}
				return stage1Idx < stage2Idx && stage2Idx < stage3Idx
			},
		},
		{
			name: "parallel dependencies",
			stages: []Stage{
				{Name: "stage1"},
				{Name: "stage2", DependsOn: []string{"stage1"}},
				{Name: "stage3", DependsOn: []string{"stage1"}},
				{Name: "stage4", DependsOn: []string{"stage2", "stage3"}},
			},
			hasError: false,
			validate: func(order []string) bool {
				// stage1 should come first, stage4 should come last
				stage1Idx, stage4Idx := -1, -1
				for i, stage := range order {
					if stage == "stage1" {
						stage1Idx = i
					} else if stage == "stage4" {
						stage4Idx = i
					}
				}
				return stage1Idx == 0 && stage4Idx == len(order)-1
			},
		},
		{
			name: "circular dependency should fail",
			stages: []Stage{
				{Name: "stage1", DependsOn: []string{"stage2"}},
				{Name: "stage2", DependsOn: []string{"stage1"}},
			},
			hasError: true,
			validate: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dg := NewDependencyGraph()

			err := dg.BuildGraph(tt.stages)
			if err != nil {
				t.Fatalf("Failed to build graph: %v", err)
			}

			order, err := dg.TopologicalSort()

			if tt.hasError && err == nil {
				t.Error("Expected error but got none")
			} else if !tt.hasError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			if !tt.hasError && tt.validate != nil {
				if !tt.validate(order) {
					t.Errorf("Topological sort validation failed for order: %v", order)
				}
			}
		})
	}
}

func TestDependencyGraph_GetBatches(t *testing.T) {
	dg := NewDependencyGraph()

	stages := []Stage{
		{Name: "stage1"},
		{Name: "stage2"},
		{Name: "stage3", DependsOn: []string{"stage1", "stage2"}},
		{Name: "stage4", DependsOn: []string{"stage1"}},
		{Name: "stage5", DependsOn: []string{"stage3", "stage4"}},
	}

	err := dg.BuildGraph(stages)
	if err != nil {
		t.Fatalf("Failed to build graph: %v", err)
	}

	batches, err := dg.GetBatches()
	if err != nil {
		t.Fatalf("GetBatches() failed: %v", err)
	}

	// Validate that dependencies are respected
	stagePosition := make(map[string]int)
	for batchIdx, batch := range batches {
		for _, stage := range batch {
			stagePosition[stage] = batchIdx
		}
	}

	for _, stage := range stages {
		stagePos := stagePosition[stage.Name]
		for _, dep := range stage.DependsOn {
			depPos := stagePosition[dep]
			if depPos >= stagePos {
				t.Errorf("Dependency violation: %s (batch %d) depends on %s (batch %d)",
					stage.Name, stagePos, dep, depPos)
			}
		}
	}

	// Verify specific expected structure for this test case
	// Batch 0: stage1, stage2 (no dependencies)
	// Batch 1: stage3, stage4 (depend on batch 0)
	// Batch 2: stage5 (depends on batch 1)
	expectedBatchCount := 3
	if len(batches) != expectedBatchCount {
		t.Errorf("Expected %d batches, got %d", expectedBatchCount, len(batches))
	}

	// Check first batch contains independent stages
	if len(batches) > 0 {
		batch0 := batches[0]
		sort.Strings(batch0)
		expected := []string{"stage1", "stage2"}
		sort.Strings(expected)
		if !reflect.DeepEqual(batch0, expected) {
			t.Errorf("First batch = %v, want %v", batch0, expected)
		}
	}
}
