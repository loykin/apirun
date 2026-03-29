package orchestrator

import (
	"testing"
)

func TestCompareValues(t *testing.T) {
	tests := []struct {
		name     string
		a        any
		b        any
		expected int
	}{
		{
			name:     "equal strings",
			a:        "hello",
			b:        "hello",
			expected: 0,
		},
		{
			name:     "a less than b",
			a:        "apple",
			b:        "banana",
			expected: -1,
		},
		{
			name:     "a greater than b",
			a:        "zebra",
			b:        "apple",
			expected: 1,
		},
		{
			name:     "equal numbers",
			a:        42,
			b:        42,
			expected: 0,
		},
		{
			name:     "number less than",
			a:        10,
			b:        20,
			expected: -1,
		},
		{
			name:     "number greater than",
			a:        30,
			b:        20,
			expected: 1,
		},
		{
			name:     "mixed types",
			a:        "10",
			b:        10,
			expected: 0, // Both convert to "10"
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := compareValues(tt.a, tt.b)
			if result != tt.expected {
				t.Errorf("compareValues(%v, %v) = %v, want %v", tt.a, tt.b, result, tt.expected)
			}
		})
	}
}

func TestOrchestrator_getStageByName(t *testing.T) {
	config := &StageOrchestration{
		Stages: []Stage{
			{Name: "stage1"},
			{Name: "stage2"},
			{Name: "stage3"},
		},
	}

	orch := NewOrchestrator(config)

	tests := []struct {
		name      string
		stageName string
		found     bool
	}{
		{
			name:      "existing stage",
			stageName: "stage2",
			found:     true,
		},
		{
			name:      "non-existing stage",
			stageName: "nonexistent",
			found:     false,
		},
		{
			name:      "empty name",
			stageName: "",
			found:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stage := orch.getStageByName(tt.stageName)

			if tt.found && stage == nil {
				t.Errorf("getStageByName(%s) expected to find stage but got nil", tt.stageName)
			}
			if !tt.found && stage != nil {
				t.Errorf("getStageByName(%s) expected nil but got stage: %v", tt.stageName, stage)
			}
			if tt.found && stage != nil && stage.Name != tt.stageName {
				t.Errorf("getStageByName(%s) returned wrong stage: %s", tt.stageName, stage.Name)
			}
		})
	}
}

func TestOrchestrator_filterStagesInRange(t *testing.T) {
	config := &StageOrchestration{}
	orch := NewOrchestrator(config)

	order := []string{"stage1", "stage2", "stage3", "stage4"}

	tests := []struct {
		name      string
		fromStage string
		toStage   string
		expected  []string
	}{
		{
			name:      "no range specified",
			fromStage: "",
			toStage:   "",
			expected:  []string{"stage1", "stage2", "stage3", "stage4"},
		},
		{
			name:      "from stage1 to stage3",
			fromStage: "stage1",
			toStage:   "stage3",
			expected:  []string{"stage1", "stage2", "stage3"},
		},
		{
			name:      "only from stage specified",
			fromStage: "stage2",
			toStage:   "",
			expected:  []string{"stage2", "stage3", "stage4"},
		},
		{
			name:      "only to stage specified",
			fromStage: "",
			toStage:   "stage2",
			expected:  []string{"stage1", "stage2"},
		},
		{
			name:      "single stage",
			fromStage: "stage2",
			toStage:   "stage2",
			expected:  []string{"stage2"},
		},
		{
			name:      "nonexistent from stage",
			fromStage: "nonexistent",
			toStage:   "stage3",
			expected:  []string{},
		},
		{
			name:      "nonexistent to stage",
			fromStage: "stage1",
			toStage:   "nonexistent",
			expected:  []string{},
		},
		{
			name:      "from after to",
			fromStage: "stage3",
			toStage:   "stage1",
			expected:  []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := orch.filterStagesInRange(order, tt.fromStage, tt.toStage)

			if len(result) != len(tt.expected) {
				t.Errorf("filterStagesInRange() length = %v, want %v", len(result), len(tt.expected))
				return
			}

			for i, stage := range result {
				if stage != tt.expected[i] {
					t.Errorf("filterStagesInRange()[%d] = %v, want %v", i, stage, tt.expected[i])
				}
			}
		})
	}
}

func TestOrchestrator_filterStagesInRangeDown(t *testing.T) {
	config := &StageOrchestration{}
	orch := NewOrchestrator(config)

	order := []string{"stage4", "stage3", "stage2", "stage1"} // Reversed order for down

	tests := []struct {
		name      string
		fromStage string
		toStage   string
		expected  []string
	}{
		{
			name:      "no range specified",
			fromStage: "",
			toStage:   "",
			expected:  []string{"stage4", "stage3", "stage2", "stage1"},
		},
		{
			name:      "from stage4 to stage2",
			fromStage: "stage4",
			toStage:   "stage2",
			expected:  []string{"stage4", "stage3"},
		},
		{
			name:      "single stage",
			fromStage: "stage3",
			toStage:   "stage3",
			expected:  []string{},
		},
		{
			name:      "from after to",
			fromStage: "stage1",
			toStage:   "stage4",
			expected:  []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := orch.filterStagesInRangeDown(order, tt.fromStage, tt.toStage)

			if len(result) != len(tt.expected) {
				t.Errorf("filterStagesInRangeDown() length = %v, want %v", len(result), len(tt.expected))
				return
			}

			for i, stage := range result {
				if stage != tt.expected[i] {
					t.Errorf("filterStagesInRangeDown()[%d] = %v, want %v", i, stage, tt.expected[i])
				}
			}
		})
	}
}

func TestOrchestrator_evaluateCondition_OSInfo(t *testing.T) {
	config := &StageOrchestration{}
	orch := NewOrchestrator(config)

	// Verify OS info is populated dynamically (not hardcoded)
	result := orch.evaluateCondition(`{{ ne .OS.GOOS "" }}`)
	if !result {
		t.Error("evaluateCondition() GOOS should be non-empty (runtime.GOOS)")
	}

	result = orch.evaluateCondition(`{{ ne .OS.GOARCH "" }}`)
	if !result {
		t.Error("evaluateCondition() GOARCH should be non-empty (runtime.GOARCH)")
	}
}
