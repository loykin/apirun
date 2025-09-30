package orchestration

import (
	"time"
)

// StageOrchestration represents the top-level configuration for stage orchestration
type StageOrchestration struct {
	APIVersion string  `yaml:"apiVersion"`
	Kind       string  `yaml:"kind"`
	Stages     []Stage `yaml:"stages"`
	Global     Global  `yaml:"global"`
}

// Stage represents a single stage in the orchestration
type Stage struct {
	Name          string            `yaml:"name"`
	ConfigPath    string            `yaml:"config_path"`
	DependsOn     []string          `yaml:"depends_on"`
	Env           map[string]string `yaml:"env"`
	EnvFromStages []EnvFromStage    `yaml:"env_from_stages"`
	Condition     string            `yaml:"condition"`
	OnFailure     string            `yaml:"on_failure"` // stop, continue, skip_dependents
	Timeout       time.Duration     `yaml:"timeout"`
	WaitBetween   time.Duration     `yaml:"wait_between"`
}

// EnvFromStage represents environment variables to inherit from other stages
type EnvFromStage struct {
	Stage string   `yaml:"stage"`
	Vars  []string `yaml:"vars"`
}

// Global represents global configuration for all stages
type Global struct {
	Env                 map[string]string `yaml:"env"`
	WaitBetweenStages   time.Duration     `yaml:"wait_between_stages"`
	RollbackOnFailure   bool              `yaml:"rollback_on_failure"`
	MaxConcurrentStages int               `yaml:"max_concurrent_stages"`
}

// StageResult represents the result of executing a stage
type StageResult struct {
	Name         string            `yaml:"name"`
	Success      bool              `yaml:"success"`
	Error        string            `yaml:"error,omitempty"`
	ExtractedEnv map[string]string `yaml:"extracted_env"`
	StartTime    time.Time         `yaml:"start_time"`
	EndTime      time.Time         `yaml:"end_time"`
	Duration     time.Duration     `yaml:"duration"`
}

// ExecutionContext holds the context for stage execution
type ExecutionContext struct {
	StageResults map[string]*StageResult
	GlobalEnv    map[string]string
}
