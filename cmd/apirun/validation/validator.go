package validation

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/loykin/apirun/cmd/apirun/config"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var ValidateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Validate migration files for syntax and structure",
	Long: `Validate migration files in the specified directory for YAML syntax errors,
required fields, and structural correctness. This command checks:
- YAML syntax validity
- Required fields (up section)
- Migration file naming convention
- Duplicate version numbers
- Request structure completeness`,
	RunE: func(cmd *cobra.Command, args []string) error {
		v := viper.GetViper()
		configPath := v.GetString("config")

		dir := ""
		if strings.TrimSpace(configPath) != "" {
			var doc config.ConfigDoc
			if err := doc.Load(configPath); err != nil {
				fmt.Printf("Warning: failed to load config file '%s': %v\n", configPath, err)
				fmt.Println("Using default migration directory...")
			} else {
				mDir := strings.TrimSpace(doc.MigrateDir)
				if mDir != "" {
					dir = mDir
				}
			}
		}

		if strings.TrimSpace(dir) == "" {
			dir = "./config/migration"
		}

		// Normalize to absolute path
		if abs, err := filepath.Abs(dir); err == nil {
			dir = abs
		}

		fmt.Printf("Validating migration files in: %s\n\n", dir)

		// Perform validation using the new modular approach
		results, err := validateMigrationFiles(dir)
		if err != nil {
			return fmt.Errorf("validation failed: %w", err)
		}

		// Print results
		printValidationResults(results)

		// Return error if validation failed
		if results.HasErrors() {
			return fmt.Errorf("validation failed with %d error(s)", results.ErrorCount())
		}

		return nil
	},
}
