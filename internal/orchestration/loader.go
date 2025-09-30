package orchestration

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// LoadStageOrchestration loads stage orchestration configuration from a YAML file
func LoadStageOrchestration(configPath string) (*StageOrchestration, error) {
	cleanPath := filepath.Clean(configPath)

	// #nosec G304 -- path is provided by user configuration
	data, err := os.ReadFile(cleanPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read stages config file %s: %w", cleanPath, err)
	}

	var orchestration StageOrchestration
	if err := yaml.Unmarshal(data, &orchestration); err != nil {
		return nil, fmt.Errorf("failed to parse stages config: %w", err)
	}

	// Set defaults
	if orchestration.APIVersion == "" {
		orchestration.APIVersion = "apirun/v1"
	}
	if orchestration.Kind == "" {
		orchestration.Kind = "StageOrchestration"
	}

	// Validate configuration
	if err := validateOrchestration(&orchestration); err != nil {
		return nil, fmt.Errorf("invalid orchestration config: %w", err)
	}

	// Resolve relative paths
	baseDir := filepath.Dir(cleanPath)
	if err := resolveConfigPaths(&orchestration, baseDir); err != nil {
		return nil, fmt.Errorf("failed to resolve config paths: %w", err)
	}

	return &orchestration, nil
}

// validateOrchestration validates the orchestration configuration
func validateOrchestration(orchestration *StageOrchestration) error {
	if len(orchestration.Stages) == 0 {
		return fmt.Errorf("no stages defined")
	}

	stageNames := make(map[string]bool)
	for i, stage := range orchestration.Stages {
		if stage.Name == "" {
			return fmt.Errorf("stage %d: name is required", i)
		}
		if stageNames[stage.Name] {
			return fmt.Errorf("duplicate stage name: %s", stage.Name)
		}
		stageNames[stage.Name] = true

		if stage.ConfigPath == "" {
			return fmt.Errorf("stage %s: config_path is required", stage.Name)
		}

		// Validate OnFailure values
		switch stage.OnFailure {
		case "", "stop", "continue", "skip_dependents":
			// Valid values
		default:
			return fmt.Errorf("stage %s: invalid on_failure value: %s (must be one of: stop, continue, skip_dependents)", stage.Name, stage.OnFailure)
		}

		// Validate dependencies exist
		for _, dep := range stage.DependsOn {
			if !stageNames[dep] && dep != stage.Name {
				// We'll validate this after all stages are processed
				// For now, just check self-dependency
				if dep == stage.Name {
					return fmt.Errorf("stage %s: cannot depend on itself", stage.Name)
				}
			}
		}
	}

	// Validate all dependencies exist
	for _, stage := range orchestration.Stages {
		for _, dep := range stage.DependsOn {
			if !stageNames[dep] {
				return fmt.Errorf("stage %s: dependency %s not found", stage.Name, dep)
			}
		}
	}

	return nil
}

// resolveConfigPaths resolves relative paths in the orchestration configuration
func resolveConfigPaths(orchestration *StageOrchestration, baseDir string) error {
	for i := range orchestration.Stages {
		stage := &orchestration.Stages[i]

		// Resolve config_path if it's relative
		if !filepath.IsAbs(stage.ConfigPath) {
			stage.ConfigPath = filepath.Join(baseDir, stage.ConfigPath)
		}

		// Check if config file exists
		if _, err := os.Stat(stage.ConfigPath); err != nil {
			return fmt.Errorf("stage %s: config file not found: %s", stage.Name, stage.ConfigPath)
		}
	}

	return nil
}
