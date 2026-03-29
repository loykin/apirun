package orchestrator

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/loykin/apirun"
	"gopkg.in/yaml.v3"
)

// StageConfig represents the configuration for a single stage
type StageConfig struct {
	MigrateDir  string              `yaml:"migrate_dir"`
	Auth        []apirun.Auth       `yaml:"auth"`
	Env         map[string]string   `yaml:"env"`
	StoreConfig *apirun.StoreConfig `yaml:"store"`
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
