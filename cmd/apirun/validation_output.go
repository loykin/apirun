package main

import (
	"fmt"
	"path/filepath"
)

// printValidationResults prints the validation results in a user-friendly format
func printValidationResults(results *ValidationResults) {
	// Print summary
	fmt.Println(results.Summary)
	fmt.Println()

	if len(results.Results) == 0 {
		return
	}

	// Group results by status
	var validFiles []ValidationResult
	var invalidFiles []ValidationResult

	for _, result := range results.Results {
		if result.Valid {
			validFiles = append(validFiles, result)
		} else {
			invalidFiles = append(invalidFiles, result)
		}
	}

	// Print invalid files first (errors)
	if len(invalidFiles) > 0 {
		fmt.Println("❌ Files with errors:")
		fmt.Println("===================")
		for _, result := range invalidFiles {
			printFileResult(result, true)
		}
		fmt.Println()
	}

	// Print files with warnings
	filesWithWarnings := 0
	for _, result := range validFiles {
		if len(result.Warnings) > 0 {
			filesWithWarnings++
		}
	}

	if filesWithWarnings > 0 {
		fmt.Println("⚠️  Files with warnings:")
		fmt.Println("=======================")
		for _, result := range validFiles {
			if len(result.Warnings) > 0 {
				printFileResult(result, false)
			}
		}
		fmt.Println()
	}

	// Print summary of valid files without warnings
	validWithoutWarnings := len(validFiles) - filesWithWarnings
	if validWithoutWarnings > 0 {
		fmt.Printf("✅ %d files are valid without warnings\n", validWithoutWarnings)
		if validWithoutWarnings <= 5 {
			// List files if there are few of them
			for _, result := range validFiles {
				if len(result.Warnings) == 0 {
					fmt.Printf("   - %s\n", filepath.Base(result.File))
				}
			}
		}
		fmt.Println()
	}

	// Print final statistics
	printStatistics(results)
}

// printFileResult prints the validation result for a single file
func printFileResult(result ValidationResult, showErrors bool) {
	filename := filepath.Base(result.File)
	fmt.Printf("📄 %s\n", filename)

	if showErrors && len(result.Errors) > 0 {
		for _, err := range result.Errors {
			fmt.Printf("   ❌ %s\n", err)
		}
	}

	if len(result.Warnings) > 0 {
		for _, warning := range result.Warnings {
			fmt.Printf("   ⚠️  %s\n", warning)
		}
	}

	fmt.Println()
}

// printStatistics prints overall validation statistics
func printStatistics(results *ValidationResults) {
	totalFiles := len(results.Results)
	errorCount := results.ErrorCount()
	warningCount := results.WarningCount()
	validFiles := 0

	for _, result := range results.Results {
		if result.Valid {
			validFiles++
		}
	}

	fmt.Println("📊 Validation Statistics:")
	fmt.Println("========================")
	fmt.Printf("Total files:      %d\n", totalFiles)
	fmt.Printf("Valid files:      %d\n", validFiles)
	fmt.Printf("Files with errors: %d\n", totalFiles-validFiles)
	fmt.Printf("Total errors:     %d\n", errorCount)
	fmt.Printf("Total warnings:   %d\n", warningCount)

	if errorCount == 0 {
		fmt.Println()
		fmt.Println("🎉 All migration files passed validation!")
	} else {
		fmt.Println()
		fmt.Printf("❗ Please fix %d error(s) before running migrations.\n", errorCount)
	}
}
