package orchestrator

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

// contextSleep sleeps for the given duration or until context is cancelled
func contextSleep(ctx context.Context, duration time.Duration) error {
	timer := time.NewTimer(duration)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

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

// GetStageResults returns a deep copy of executed stage results
func (o *Orchestrator) GetStageResults() map[string]*StageResult {
	o.mu.RLock()
	defer o.mu.RUnlock()

	results := make(map[string]*StageResult, len(o.context.StageResults))
	for k, v := range o.context.StageResults {
		copyValue := *v
		results[k] = &copyValue
	}
	return results
}
