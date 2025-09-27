package main

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
)

var validateCmd = &cobra.Command{
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
			var doc ConfigDoc
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

		fmt.Printf("Validating migration files in: %s\n", dir)

		results, err := validateMigrationFiles(dir)
		if err != nil {
			return fmt.Errorf("validation failed: %w", err)
		}

		printValidationResults(results)

		// Return error if any validations failed
		if results.HasErrors() {
			return fmt.Errorf("validation completed with %d error(s)", results.ErrorCount())
		}

		fmt.Println("\nAll migration files are valid!")
		return nil
	},
}

type ValidationResult struct {
	FileName string
	Version  int
	Errors   []string
	Warnings []string
}

type ValidationResults struct {
	Results         []ValidationResult
	DuplicateErrors []string
}

func (vr *ValidationResults) HasErrors() bool {
	if len(vr.DuplicateErrors) > 0 {
		return true
	}
	for _, result := range vr.Results {
		if len(result.Errors) > 0 {
			return true
		}
	}
	return false
}

func (vr *ValidationResults) ErrorCount() int {
	count := len(vr.DuplicateErrors)
	for _, result := range vr.Results {
		count += len(result.Errors)
	}
	return count
}

func validateMigrationFiles(dir string) (*ValidationResults, error) {
	results := &ValidationResults{}

	// Check if directory exists
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return nil, fmt.Errorf("migration directory does not exist: %s", dir)
	}

	// Find all migration files
	migrationFiles, err := findMigrationFiles(dir)
	if err != nil {
		return nil, err
	}

	if len(migrationFiles) == 0 {
		fmt.Printf("No migration files found in %s\n", dir)
		return results, nil
	}

	// Track versions to detect duplicates
	versionMap := make(map[int][]string)

	// Validate each file
	for _, file := range migrationFiles {
		result := validateSingleFile(filepath.Join(dir, file))
		results.Results = append(results.Results, result)

		if result.Version > 0 {
			versionMap[result.Version] = append(versionMap[result.Version], file)
		}
	}

	// Check for duplicate versions
	for version, files := range versionMap {
		if len(files) > 1 {
			results.DuplicateErrors = append(results.DuplicateErrors,
				fmt.Sprintf("Duplicate version %d found in files: %s", version, strings.Join(files, ", ")))
		}
	}

	return results, nil
}

func findMigrationFiles(dir string) ([]string, error) {
	var files []string

	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() {
			return nil
		}

		// Check if it's a YAML migration file
		if strings.HasSuffix(strings.ToLower(d.Name()), ".yaml") || strings.HasSuffix(strings.ToLower(d.Name()), ".yml") {
			relPath, err := filepath.Rel(dir, path)
			if err != nil {
				return err
			}
			files = append(files, relPath)
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	sort.Strings(files)
	return files, nil
}

func validateSingleFile(filePath string) ValidationResult {
	result := ValidationResult{
		FileName: filepath.Base(filePath),
	}

	// Read file
	content, err := os.ReadFile(filePath)
	if err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("Failed to read file: %v", err))
		return result
	}

	// Parse version from filename using the same regex as the internal migration package
	versionFileRegex := regexp.MustCompile(`^(\d+)_.*\.(ya?ml)$`)
	matches := versionFileRegex.FindStringSubmatch(result.FileName)
	if len(matches) == 0 {
		result.Errors = append(result.Errors, "Invalid migration filename format - must be '{version}_{name}.yaml'")
	} else {
		var version int
		if _, err := fmt.Sscanf(matches[1], "%d", &version); err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("Invalid version number in filename: %v", err))
		} else {
			result.Version = version
		}
	}

	// Parse YAML
	var migration map[string]interface{}
	if err := yaml.Unmarshal(content, &migration); err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("Invalid YAML syntax: %v", err))
		return result
	}

	// Validate structure
	validateMigrationStructure(migration, &result)

	return result
}

func validateMigrationStructure(migration map[string]interface{}, result *ValidationResult) {
	// Check for required 'up' section
	up, hasUp := migration["up"]
	if !hasUp {
		result.Errors = append(result.Errors, "Missing required 'up' section")
		return
	}

	// Validate up section
	if upMap, ok := up.(map[string]interface{}); ok {
		validateUpSection(upMap, result)
	} else {
		result.Errors = append(result.Errors, "'up' section must be an object")
	}

	// Validate down section if present
	if down, hasDown := migration["down"]; hasDown {
		if downMap, ok := down.(map[string]interface{}); ok {
			validateDownSection(downMap, result)
		} else {
			result.Errors = append(result.Errors, "'down' section must be an object")
		}
	} else {
		result.Warnings = append(result.Warnings, "No 'down' section found - rollback will not be possible")
	}
}

func validateUpSection(up map[string]interface{}, result *ValidationResult) {
	// Check for name field
	if _, hasName := up["name"]; !hasName {
		result.Warnings = append(result.Warnings, "'up' section missing 'name' field")
	}

	// Check for request section
	request, hasRequest := up["request"]
	if !hasRequest {
		result.Errors = append(result.Errors, "'up' section missing required 'request' field")
		return
	}

	if requestMap, ok := request.(map[string]interface{}); ok {
		validateRequestSection(requestMap, result, "up.request")
	} else {
		result.Errors = append(result.Errors, "'up.request' must be an object")
	}

	// Validate response section if present
	if response, hasResponse := up["response"]; hasResponse {
		if responseMap, ok := response.(map[string]interface{}); ok {
			validateResponseSection(responseMap, result, "up.response")
		} else {
			result.Errors = append(result.Errors, "'up.response' must be an object")
		}
	}
}

func validateDownSection(down map[string]interface{}, result *ValidationResult) {
	// Check if it has direct HTTP fields or a request section
	hasMethod := false
	hasURL := false
	hasRequest := false

	if _, ok := down["method"]; ok {
		hasMethod = true
	}
	if _, ok := down["url"]; ok {
		hasURL = true
	}
	if request, ok := down["request"]; ok {
		hasRequest = true
		if requestMap, ok := request.(map[string]interface{}); ok {
			validateRequestSection(requestMap, result, "down.request")
		} else {
			result.Errors = append(result.Errors, "'down.request' must be an object")
		}
	}

	// Validate that we have either direct fields or request section
	if hasRequest {
		if hasMethod || hasURL {
			result.Warnings = append(result.Warnings, "'down' section has both direct HTTP fields and 'request' section - 'request' takes precedence")
		}
	} else if !hasMethod && !hasURL {
		result.Errors = append(result.Errors, "'down' section must have either 'method'/'url' fields or a 'request' section")
	} else if !hasMethod || !hasURL {
		if !hasMethod {
			result.Warnings = append(result.Warnings, "'down' section missing 'method' field")
		}
		if !hasURL {
			result.Errors = append(result.Errors, "'down' section missing required 'url' field")
		}
	}

	// Validate find section if present
	if find, hasFind := down["find"]; hasFind {
		if findMap, ok := find.(map[string]interface{}); ok {
			validateFindSection(findMap, result)
		} else {
			result.Errors = append(result.Errors, "'down.find' must be an object")
		}
	}
}

func validateRequestSection(request map[string]interface{}, result *ValidationResult, prefix string) {
	// Check required fields
	if _, hasMethod := request["method"]; !hasMethod {
		result.Warnings = append(result.Warnings, fmt.Sprintf("'%s' missing 'method' field", prefix))
	}

	if _, hasURL := request["url"]; !hasURL {
		result.Errors = append(result.Errors, fmt.Sprintf("'%s' missing required 'url' field", prefix))
	}

	// Validate headers if present
	if headers, hasHeaders := request["headers"]; hasHeaders {
		if headersList, ok := headers.([]interface{}); ok {
			for i, header := range headersList {
				if headerMap, ok := header.(map[string]interface{}); ok {
					if _, hasName := headerMap["name"]; !hasName {
						result.Errors = append(result.Errors, fmt.Sprintf("'%s.headers[%d]' missing 'name' field", prefix, i))
					}
					if _, hasValue := headerMap["value"]; !hasValue {
						result.Errors = append(result.Errors, fmt.Sprintf("'%s.headers[%d]' missing 'value' field", prefix, i))
					}
				} else {
					result.Errors = append(result.Errors, fmt.Sprintf("'%s.headers[%d]' must be an object with 'name' and 'value' fields", prefix, i))
				}
			}
		} else {
			result.Errors = append(result.Errors, fmt.Sprintf("'%s.headers' must be an array", prefix))
		}
	}

	// Validate queries if present
	if queries, hasQueries := request["queries"]; hasQueries {
		if queriesList, ok := queries.([]interface{}); ok {
			for i, query := range queriesList {
				if queryMap, ok := query.(map[string]interface{}); ok {
					if _, hasName := queryMap["name"]; !hasName {
						result.Errors = append(result.Errors, fmt.Sprintf("'%s.queries[%d]' missing 'name' field", prefix, i))
					}
					if _, hasValue := queryMap["value"]; !hasValue {
						result.Errors = append(result.Errors, fmt.Sprintf("'%s.queries[%d]' missing 'value' field", prefix, i))
					}
				} else {
					result.Errors = append(result.Errors, fmt.Sprintf("'%s.queries[%d]' must be an object with 'name' and 'value' fields", prefix, i))
				}
			}
		} else {
			result.Errors = append(result.Errors, fmt.Sprintf("'%s.queries' must be an array", prefix))
		}
	}
}

func validateResponseSection(response map[string]interface{}, result *ValidationResult, prefix string) {
	// Validate env_missing if present
	if envMissing, hasEnvMissing := response["env_missing"]; hasEnvMissing {
		if envMissingStr, ok := envMissing.(string); ok {
			if envMissingStr != "skip" && envMissingStr != "fail" {
				result.Errors = append(result.Errors, fmt.Sprintf("'%s.env_missing' must be either 'skip' or 'fail'", prefix))
			}
		} else {
			result.Errors = append(result.Errors, fmt.Sprintf("'%s.env_missing' must be a string", prefix))
		}
	}

	// Validate result_code if present
	if resultCode, hasResultCode := response["result_code"]; hasResultCode {
		if resultCodeList, ok := resultCode.([]interface{}); ok {
			for i, code := range resultCodeList {
				if _, ok := code.(string); !ok {
					if _, ok := code.(int); !ok {
						result.Errors = append(result.Errors, fmt.Sprintf("'%s.result_code[%d]' must be a string or number", prefix, i))
					}
				}
			}
		} else {
			result.Errors = append(result.Errors, fmt.Sprintf("'%s.result_code' must be an array", prefix))
		}
	}
}

func validateFindSection(find map[string]interface{}, result *ValidationResult) {
	// Check for request section
	request, hasRequest := find["request"]
	if !hasRequest {
		result.Errors = append(result.Errors, "'down.find' section missing required 'request' field")
		return
	}

	if requestMap, ok := request.(map[string]interface{}); ok {
		validateRequestSection(requestMap, result, "down.find.request")
	} else {
		result.Errors = append(result.Errors, "'down.find.request' must be an object")
	}

	// Validate response section if present
	if response, hasResponse := find["response"]; hasResponse {
		if responseMap, ok := response.(map[string]interface{}); ok {
			validateResponseSection(responseMap, result, "down.find.response")
		} else {
			result.Errors = append(result.Errors, "'down.find.response' must be an object")
		}
	}
}

func printValidationResults(results *ValidationResults) {
	fmt.Println()

	// Print duplicate version errors first
	if len(results.DuplicateErrors) > 0 {
		fmt.Println("🔴 Duplicate Version Errors:")
		for _, err := range results.DuplicateErrors {
			fmt.Printf("  ❌ %s\n", err)
		}
		fmt.Println()
	}

	// Print results for each file
	for _, result := range results.Results {
		if len(result.Errors) > 0 || len(result.Warnings) > 0 {
			fmt.Printf("📄 %s (version %d):\n", result.FileName, result.Version)

			for _, err := range result.Errors {
				fmt.Printf("  ❌ Error: %s\n", err)
			}

			for _, warning := range result.Warnings {
				fmt.Printf("  ⚠️  Warning: %s\n", warning)
			}

			fmt.Println()
		} else {
			fmt.Printf("✅ %s (version %d): Valid\n", result.FileName, result.Version)
		}
	}

	// Print summary
	totalFiles := len(results.Results)
	errorFiles := 0
	warningFiles := 0
	validFiles := 0

	for _, result := range results.Results {
		if len(result.Errors) > 0 {
			errorFiles++
		} else if len(result.Warnings) > 0 {
			warningFiles++
		} else {
			validFiles++
		}
	}

	fmt.Printf("\n📊 Validation Summary:\n")
	fmt.Printf("  Total files: %d\n", totalFiles)
	fmt.Printf("  ✅ Valid files: %d\n", validFiles)
	fmt.Printf("  ⚠️  Files with warnings: %d\n", warningFiles)
	fmt.Printf("  ❌ Files with errors: %d\n", errorFiles)

	if len(results.DuplicateErrors) > 0 {
		fmt.Printf("  🔴 Duplicate version errors: %d\n", len(results.DuplicateErrors))
	}
}

func init() {
	// Add validation-specific flags if needed
	// validateCmd.Flags().BoolVar(&someFlag, "some-flag", false, "Some description")
}
