package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateMigrationFiles(t *testing.T) {
	// Create temporary directory for test files
	tmpDir, err := os.MkdirTemp("", "apirun_validate_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// Test case 1: Valid migration file
	validContent := `up:
  name: create user
  request:
    method: POST
    url: "https://api.example.com/users"
    headers:
      Content-Type: application/json
  response:
    result_code: ["201"]

down:
  name: delete user
  request:
    method: DELETE
    url: "https://api.example.com/users/{{.user_id}}"
`
	validFile := filepath.Join(tmpDir, "001_create_user.yaml")
	if err := os.WriteFile(validFile, []byte(validContent), 0644); err != nil {
		t.Fatalf("Failed to write valid file: %v", err)
	}

	// Test case 2: Invalid YAML syntax
	invalidYamlContent := `up:
  name: invalid yaml
  request:
    method: POST
    - invalid syntax here
`
	invalidYamlFile := filepath.Join(tmpDir, "002_invalid_yaml.yaml")
	if err := os.WriteFile(invalidYamlFile, []byte(invalidYamlContent), 0644); err != nil {
		t.Fatalf("Failed to write invalid yaml file: %v", err)
	}

	// Test case 3: Missing required 'up' section
	missingUpContent := `down:
  name: only down section
  request:
    method: DELETE
    url: "https://api.example.com/users/1"
`
	missingUpFile := filepath.Join(tmpDir, "003_missing_up.yaml")
	if err := os.WriteFile(missingUpFile, []byte(missingUpContent), 0644); err != nil {
		t.Fatalf("Failed to write missing up file: %v", err)
	}

	// Test case 4: Invalid filename (should be ignored)
	invalidFilename := filepath.Join(tmpDir, "invalid_filename.yaml")
	if err := os.WriteFile(invalidFilename, []byte(validContent), 0644); err != nil {
		t.Fatalf("Failed to write invalid filename file: %v", err)
	}

	// Run validation
	results, err := validateMigrationFiles(tmpDir)
	if err != nil {
		t.Fatalf("Validation failed: %v", err)
	}

	// Check that we found the expected number of files (should ignore invalid filename)
	expectedFiles := 3
	if len(results.Results) != expectedFiles {
		t.Errorf("Expected %d files, got %d", expectedFiles, len(results.Results))
	}

	// Check individual results
	foundValidFile := false
	foundInvalidYaml := false
	foundMissingUp := false

	for _, result := range results.Results {
		filename := filepath.Base(result.File)

		switch filename {
		case "001_create_user.yaml":
			foundValidFile = true
			if !result.Valid {
				t.Errorf("Expected valid file to be marked as valid")
			}
			if len(result.Errors) != 0 {
				t.Errorf("Expected valid file to have no errors, got: %v", result.Errors)
			}
		case "002_invalid_yaml.yaml":
			foundInvalidYaml = true
			if result.Valid {
				t.Error("Expected invalid YAML file to be marked as invalid")
			}
			if len(result.Errors) == 0 {
				t.Error("Expected invalid YAML file to have errors")
			}
		case "003_missing_up.yaml":
			foundMissingUp = true
			if result.Valid {
				t.Error("Expected file missing 'up' section to be marked as invalid")
			}
			if len(result.Errors) == 0 {
				t.Error("Expected file missing 'up' section to have errors")
			}
		}
	}

	if !foundValidFile {
		t.Error("Valid file not found in results")
	}
	if !foundInvalidYaml {
		t.Error("Invalid YAML file not found in results")
	}
	if !foundMissingUp {
		t.Error("File missing 'up' section not found in results")
	}

	// Check overall validation status
	if !results.HasErrors() {
		t.Error("Expected validation to have errors due to invalid files")
	}
}

func TestValidateSingleFile_ValidFile(t *testing.T) {
	// Create temporary file
	tmpDir, err := os.MkdirTemp("", "apirun_single_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	content := `up:
  name: test migration
  request:
    method: GET
    url: "https://api.example.com/test"
  response:
    result_code: ["200"]
`

	filePath := filepath.Join(tmpDir, "001_test.yaml")
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	result := validateSingleFile(filePath)

	if !result.Valid {
		t.Errorf("Expected file to be valid, got invalid")
	}
	if len(result.Errors) > 0 {
		t.Errorf("Expected no errors, got: %v", result.Errors)
	}
	if result.File != filePath {
		t.Errorf("Expected file path %s, got %s", filePath, result.File)
	}
}

func TestValidateSingleFile_InvalidFile(t *testing.T) {
	// Create temporary file with invalid content
	tmpDir, err := os.MkdirTemp("", "apirun_invalid_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	content := `invalid yaml content: [
  - missing closing bracket
`

	filePath := filepath.Join(tmpDir, "001_invalid.yaml")
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	result := validateSingleFile(filePath)

	if result.Valid {
		t.Error("Expected file to be invalid")
	}
	if len(result.Errors) == 0 {
		t.Error("Expected errors for invalid file")
	}
}

func TestFindMigrationFiles(t *testing.T) {
	// Create temporary directory
	tmpDir, err := os.MkdirTemp("", "apirun_find_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// Create valid migration files
	validFiles := []string{
		"001_first.yaml",
		"002_second.yml",
		"010_tenth.yaml",
	}

	for _, filename := range validFiles {
		filePath := filepath.Join(tmpDir, filename)
		if err := os.WriteFile(filePath, []byte("up:\n  name: test"), 0644); err != nil {
			t.Fatalf("Failed to create test file %s: %v", filename, err)
		}
	}

	// Create files that should be ignored
	ignoredFiles := []string{
		"invalid_name.yaml",
		"1_no_leading_zeros.yaml",
		"001_valid.txt",
		"README.md",
	}

	for _, filename := range ignoredFiles {
		filePath := filepath.Join(tmpDir, filename)
		if err := os.WriteFile(filePath, []byte("content"), 0644); err != nil {
			t.Fatalf("Failed to create ignored file %s: %v", filename, err)
		}
	}

	// Find migration files
	files, err := findMigrationFiles(tmpDir)
	if err != nil {
		t.Fatalf("Failed to find migration files: %v", err)
	}

	// Check that we found the expected number of files
	if len(files) != len(validFiles) {
		t.Errorf("Expected %d files, got %d", len(validFiles), len(files))
	}

	// Check that files are sorted
	for i := 0; i < len(files)-1; i++ {
		if files[i] >= files[i+1] {
			t.Error("Files are not properly sorted")
		}
	}

	// Check that all valid files are found
	for _, expectedFile := range validFiles {
		found := false
		for _, foundFile := range files {
			if strings.HasSuffix(foundFile, expectedFile) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected file %s not found in results", expectedFile)
		}
	}
}

func TestValidationResult_Methods(t *testing.T) {
	results := &ValidationResults{}

	// Test empty results
	if results.HasErrors() {
		t.Error("Empty results should not have errors")
	}
	if results.ErrorCount() != 0 {
		t.Errorf("Expected 0 errors, got %d", results.ErrorCount())
	}
	if results.WarningCount() != 0 {
		t.Errorf("Expected 0 warnings, got %d", results.WarningCount())
	}

	// Add some results
	results.AddResult(ValidationResult{
		File:   "test1.yaml",
		Errors: []string{"error1", "error2"},
		Valid:  false,
	})

	results.AddResult(ValidationResult{
		File:     "test2.yaml",
		Warnings: []string{"warning1"},
		Valid:    true,
	})

	// Test with results
	if !results.HasErrors() {
		t.Error("Results should have errors")
	}
	if results.ErrorCount() != 2 {
		t.Errorf("Expected 2 errors, got %d", results.ErrorCount())
	}
	if results.WarningCount() != 1 {
		t.Errorf("Expected 1 warning, got %d", results.WarningCount())
	}
}
