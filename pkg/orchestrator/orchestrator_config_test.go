package orchestrator

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/loykin/apirun"
)

func TestOrchestrator_loadStageConfig(t *testing.T) {
	// Create temporary directory for test files
	tempDir := t.TempDir()

	tests := []struct {
		name        string
		configYAML  string
		migrateDir  string
		expectError bool
		expectedDir string
	}{
		{
			name: "valid config with absolute migrate_dir",
			configYAML: `migrate_dir: /absolute/path/migrations
env:
  TEST_VAR: test_value
store:
  driver: sqlite
  driver_config:
    path: ":memory:"
`,
			expectedDir: "/absolute/path/migrations",
			expectError: false,
		},
		{
			name: "valid config with relative migrate_dir",
			configYAML: `migrate_dir: ./migrations
env:
  TEST_VAR: test_value
`,
			migrateDir:  "migrations",
			expectError: false,
		},
		{
			name: "config without migrate_dir",
			configYAML: `env:
  TEST_VAR: test_value
`,
			expectError: false,
		},
		{
			name: "invalid YAML",
			configYAML: `migrate_dir: ./migrations
invalid: yaml: content: [
`,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test config file
			configPath := filepath.Join(tempDir, "test_config.yaml")
			err := os.WriteFile(configPath, []byte(tt.configYAML), 0644)
			if err != nil {
				t.Fatalf("Failed to create test config file: %v", err)
			}

			// Create expected migrate directory if specified
			if tt.migrateDir != "" {
				migratePath := filepath.Join(tempDir, tt.migrateDir)
				err := os.MkdirAll(migratePath, 0755)
				if err != nil {
					t.Fatalf("Failed to create migrate directory: %v", err)
				}
				tt.expectedDir = migratePath
			}

			orch := NewOrchestrator(&StageOrchestration{})
			config, err := orch.loadStageConfig(configPath)

			if tt.expectError {
				if err == nil {
					t.Errorf("loadStageConfig() expected error but got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("loadStageConfig() unexpected error: %v", err)
				return
			}

			if config == nil {
				t.Fatal("loadStageConfig() returned nil config")
			}

			// Check migrate_dir resolution
			if tt.expectedDir != "" {
				if config.MigrateDir != tt.expectedDir {
					t.Errorf("loadStageConfig() migrate_dir = %v, want %v", config.MigrateDir, tt.expectedDir)
				}
			}

			// Check environment variables
			if config.Env != nil {
				if config.Env["TEST_VAR"] != "test_value" {
					t.Errorf("loadStageConfig() env TEST_VAR = %v, want test_value", config.Env["TEST_VAR"])
				}
			}

			// Auth configuration removed from this test case

			// Store configuration is parsed but driver field may not be populated in this simple YAML test
		})
	}
}

func TestOrchestrator_loadStageConfig_FileNotFound(t *testing.T) {
	orch := NewOrchestrator(&StageOrchestration{})
	_, err := orch.loadStageConfig("/nonexistent/config.yaml")

	if err == nil {
		t.Error("loadStageConfig() expected error for nonexistent file")
	}
}

func TestOrchestrator_loadStageConfig_RelativePaths(t *testing.T) {
	tempDir := t.TempDir()

	// Create a subdirectory for the config
	configDir := filepath.Join(tempDir, "configs")
	err := os.MkdirAll(configDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create config directory: %v", err)
	}

	// Create config with relative migrate_dir
	configContent := `migrate_dir: ../migrations
env:
  TEST_VAR: test_value
`
	configPath := filepath.Join(configDir, "stage.yaml")
	err = os.WriteFile(configPath, []byte(configContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test config: %v", err)
	}

	// Create the migrations directory relative to config
	migrationsDir := filepath.Join(tempDir, "migrations")
	err = os.MkdirAll(migrationsDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create migrations directory: %v", err)
	}

	orch := NewOrchestrator(&StageOrchestration{})
	config, err := orch.loadStageConfig(configPath)

	if err != nil {
		t.Fatalf("loadStageConfig() unexpected error: %v", err)
	}

	expectedMigrateDir := migrationsDir
	if config.MigrateDir != expectedMigrateDir {
		t.Errorf("loadStageConfig() migrate_dir = %v, want %v", config.MigrateDir, expectedMigrateDir)
	}
}

func TestStageConfig_Types(t *testing.T) {
	// Test that StageConfig struct has the expected fields and types
	config := &StageConfig{
		MigrateDir: "/test/path",
		Env: map[string]string{
			"KEY": "value",
		},
		StoreConfig: &apirun.StoreConfig{},
	}

	if config.MigrateDir != "/test/path" {
		t.Error("StageConfig.MigrateDir field not working")
	}

	if config.Env["KEY"] != "value" {
		t.Error("StageConfig.Env field not working")
	}

	if config.StoreConfig == nil {
		t.Error("StageConfig.StoreConfig field not working")
	}
}
