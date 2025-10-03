package validation

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"

	"gopkg.in/yaml.v3"
)

// validateMigrationFiles validates all migration files in the specified directory
func validateMigrationFiles(dir string) (*ValidationResults, error) {
	results := &ValidationResults{}

	// Check if directory exists
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return nil, fmt.Errorf("migration directory does not exist: %s", dir)
	}

	// Find all migration files
	files, err := findMigrationFiles(dir)
	if err != nil {
		return nil, fmt.Errorf("failed to find migration files: %w", err)
	}

	if len(files) == 0 {
		results.Summary = fmt.Sprintf("No migration files found in directory: %s", dir)
		return results, nil
	}

	// Validate each file
	var allErrors []string
	var allWarnings []string

	for _, filePath := range files {
		result := validateSingleFile(filePath)
		results.AddResult(result)

		allErrors = append(allErrors, result.Errors...)
		allWarnings = append(allWarnings, result.Warnings...)
	}

	// Generate summary
	errorCount := len(allErrors)
	warningCount := len(allWarnings)
	fileCount := len(files)

	if errorCount == 0 && warningCount == 0 {
		results.Summary = fmt.Sprintf("✅ All %d migration files are valid", fileCount)
	} else {
		results.Summary = fmt.Sprintf("Validation completed for %d files: %d errors, %d warnings",
			fileCount, errorCount, warningCount)
	}

	return results, nil
}

// findMigrationFiles discovers migration files in the specified directory
func findMigrationFiles(dir string) ([]string, error) {
	var files []string

	// Migration file pattern: 001_name.yaml, 002_name.yml, etc.
	migrationPattern := regexp.MustCompile(`^\d{3}_.*\.ya?ml$`)

	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Skip directories
		if d.IsDir() {
			return nil
		}

		// Check if filename matches migration pattern
		filename := d.Name()
		if migrationPattern.MatchString(filename) {
			files = append(files, path)
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	// Sort files by name to ensure consistent order
	sort.Strings(files)

	return files, nil
}

// validateSingleFile validates a single migration file
func validateSingleFile(filePath string) ValidationResult {
	result := ValidationResult{
		File:  filePath,
		Valid: true,
	}

	// Read file content
	content, err := os.ReadFile(filePath)
	if err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("Failed to read file: %v", err))
		result.Valid = false
		return result
	}

	// Parse YAML content
	var migration map[string]interface{}
	if err := yaml.Unmarshal(content, &migration); err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("Invalid YAML syntax: %v", err))
		result.Valid = false
		return result
	}

	// Validate migration structure
	validateMigrationStructure(migration, &result)

	// Set final validity based on errors
	if len(result.Errors) > 0 {
		result.Valid = false
	}

	return result
}
