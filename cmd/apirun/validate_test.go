package main

import (
	"os"
	"path/filepath"
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
      - name: Content-Type
        value: application/json
  response:
    result_code: ["201"]

down:
  name: delete user
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

	// Test case 3: Missing up section
	missingUpContent := `down:
  name: only down section
  method: DELETE
  url: "https://api.example.com/users/123"
`
	missingUpFile := filepath.Join(tmpDir, "003_missing_up.yaml")
	if err := os.WriteFile(missingUpFile, []byte(missingUpContent), 0644); err != nil {
		t.Fatalf("Failed to write missing up file: %v", err)
	}

	// Test case 4: Invalid filename
	invalidFilenameContent := `up:
  name: invalid filename
  request:
    method: GET
    url: "https://api.example.com/test"
`
	invalidFilenameFile := filepath.Join(tmpDir, "invalid_filename.yaml")
	if err := os.WriteFile(invalidFilenameFile, []byte(invalidFilenameContent), 0644); err != nil {
		t.Fatalf("Failed to write invalid filename file: %v", err)
	}

	// Test case 5: Duplicate version
	duplicateContent := `up:
  name: duplicate version
  request:
    method: GET
    url: "https://api.example.com/test"
`
	duplicateFile1 := filepath.Join(tmpDir, "004_duplicate.yaml")
	duplicateFile2 := filepath.Join(tmpDir, "004_another_duplicate.yaml")
	if err := os.WriteFile(duplicateFile1, []byte(duplicateContent), 0644); err != nil {
		t.Fatalf("Failed to write duplicate file 1: %v", err)
	}
	if err := os.WriteFile(duplicateFile2, []byte(duplicateContent), 0644); err != nil {
		t.Fatalf("Failed to write duplicate file 2: %v", err)
	}

	// Run validation
	results, err := validateMigrationFiles(tmpDir)
	if err != nil {
		t.Fatalf("Validation failed: %v", err)
	}

	// Verify results
	if !results.HasErrors() {
		t.Error("Expected validation to have errors, but it didn't")
	}

	errorCount := results.ErrorCount()
	if errorCount == 0 {
		t.Error("Expected at least one error, but got none")
	}

	// Check for specific errors
	foundValidFile := false
	foundInvalidYaml := false
	foundMissingUp := false
	foundInvalidFilename := false
	foundDuplicateError := false

	for _, result := range results.Results {
		switch result.FileName {
		case "001_create_user.yaml":
			foundValidFile = true
			if len(result.Errors) != 0 {
				t.Errorf("Expected valid file to have no errors, got: %v", result.Errors)
			}
			if result.Version != 1 {
				t.Errorf("Expected version 1, got %d", result.Version)
			}
		case "002_invalid_yaml.yaml":
			foundInvalidYaml = true
			if len(result.Errors) == 0 {
				t.Error("Expected invalid YAML file to have errors")
			}
		case "003_missing_up.yaml":
			foundMissingUp = true
			if len(result.Errors) == 0 {
				t.Error("Expected missing up file to have errors")
			}
		case "invalid_filename.yaml":
			foundInvalidFilename = true
			if len(result.Errors) == 0 {
				t.Error("Expected invalid filename file to have errors")
			}
		}
	}

	if len(results.DuplicateErrors) > 0 {
		foundDuplicateError = true
	}

	if !foundValidFile {
		t.Error("Valid file result not found")
	}
	if !foundInvalidYaml {
		t.Error("Invalid YAML file result not found")
	}
	if !foundMissingUp {
		t.Error("Missing up file result not found")
	}
	if !foundInvalidFilename {
		t.Error("Invalid filename file result not found")
	}
	if !foundDuplicateError {
		t.Error("Duplicate version error not found")
	}
}

func TestValidateSingleFile(t *testing.T) {
	// Create temporary file for testing
	tmpDir, err := os.MkdirTemp("", "apirun_validate_single_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// Test valid file
	validContent := `up:
  name: test migration
  request:
    method: GET
    url: "https://api.example.com/test"
  response:
    result_code: ["200"]
`
	validFile := filepath.Join(tmpDir, "001_test.yaml")
	if err := os.WriteFile(validFile, []byte(validContent), 0644); err != nil {
		t.Fatalf("Failed to write valid file: %v", err)
	}

	result := validateSingleFile(validFile)

	if result.FileName != "001_test.yaml" {
		t.Errorf("Expected filename '001_test.yaml', got '%s'", result.FileName)
	}
	if result.Version != 1 {
		t.Errorf("Expected version 1, got %d", result.Version)
	}
	if len(result.Errors) != 0 {
		t.Errorf("Expected no errors for valid file, got: %v", result.Errors)
	}
}

func TestValidateMigrationStructure(t *testing.T) {
	tests := []struct {
		name           string
		migration      map[string]interface{}
		expectErrors   int
		expectWarnings int
	}{
		{
			name: "valid migration",
			migration: map[string]interface{}{
				"up": map[string]interface{}{
					"name": "test",
					"request": map[string]interface{}{
						"method": "GET",
						"url":    "https://api.example.com/test",
					},
				},
				"down": map[string]interface{}{
					"method": "DELETE",
					"url":    "https://api.example.com/test/{{.id}}",
				},
			},
			expectErrors:   0,
			expectWarnings: 0,
		},
		{
			name: "missing up section",
			migration: map[string]interface{}{
				"down": map[string]interface{}{
					"method": "DELETE",
					"url":    "https://api.example.com/test/{{.id}}",
				},
			},
			expectErrors:   1,
			expectWarnings: 0,
		},
		{
			name: "missing request in up",
			migration: map[string]interface{}{
				"up": map[string]interface{}{
					"name": "test",
				},
			},
			expectErrors:   1,
			expectWarnings: 1, // no down section
		},
		{
			name: "no down section",
			migration: map[string]interface{}{
				"up": map[string]interface{}{
					"name": "test",
					"request": map[string]interface{}{
						"method": "GET",
						"url":    "https://api.example.com/test",
					},
				},
			},
			expectErrors:   0,
			expectWarnings: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ValidationResult{}
			validateMigrationStructure(tt.migration, &result)

			if len(result.Errors) != tt.expectErrors {
				t.Errorf("Expected %d errors, got %d: %v", tt.expectErrors, len(result.Errors), result.Errors)
			}
			if len(result.Warnings) != tt.expectWarnings {
				t.Errorf("Expected %d warnings, got %d: %v", tt.expectWarnings, len(result.Warnings), result.Warnings)
			}
		})
	}
}

func TestValidateRequestSection(t *testing.T) {
	tests := []struct {
		name         string
		request      map[string]interface{}
		expectErrors int
	}{
		{
			name: "valid request",
			request: map[string]interface{}{
				"method": "POST",
				"url":    "https://api.example.com/test",
				"headers": []interface{}{
					map[string]interface{}{
						"name":  "Content-Type",
						"value": "application/json",
					},
				},
			},
			expectErrors: 0,
		},
		{
			name: "missing url",
			request: map[string]interface{}{
				"method": "POST",
			},
			expectErrors: 1,
		},
		{
			name: "invalid headers format",
			request: map[string]interface{}{
				"method": "POST",
				"url":    "https://api.example.com/test",
				"headers": []interface{}{
					map[string]interface{}{
						"name": "Content-Type",
						// missing value
					},
				},
			},
			expectErrors: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ValidationResult{}
			validateRequestSection(tt.request, &result, "test")

			if len(result.Errors) != tt.expectErrors {
				t.Errorf("Expected %d errors, got %d: %v", tt.expectErrors, len(result.Errors), result.Errors)
			}
		})
	}
}

func TestFindMigrationFiles(t *testing.T) {
	// Create temporary directory for test files
	tmpDir, err := os.MkdirTemp("", "apirun_find_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// Create test files
	testFiles := []string{
		"001_first.yaml",
		"002_second.yml",
		"003_third.yaml",
		"not_a_migration.txt",
		"004_fourth.yaml",
	}

	for _, filename := range testFiles {
		file := filepath.Join(tmpDir, filename)
		if err := os.WriteFile(file, []byte("test content"), 0644); err != nil {
			t.Fatalf("Failed to write test file %s: %v", filename, err)
		}
	}

	// Find migration files
	files, err := findMigrationFiles(tmpDir)
	if err != nil {
		t.Fatalf("Failed to find migration files: %v", err)
	}

	// Verify results
	expectedFiles := []string{
		"001_first.yaml",
		"002_second.yml",
		"003_third.yaml",
		"004_fourth.yaml",
	}

	if len(files) != len(expectedFiles) {
		t.Errorf("Expected %d files, got %d: %v", len(expectedFiles), len(files), files)
	}

	for i, expected := range expectedFiles {
		if i >= len(files) || files[i] != expected {
			t.Errorf("Expected file %d to be '%s', got '%s'", i, expected, files[i])
		}
	}
}
