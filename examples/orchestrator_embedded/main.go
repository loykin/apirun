package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/loykin/apirun/pkg/orchestrator"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "--dry-run" {
		runDryRun()
		return
	}

	if len(os.Args) > 1 && os.Args[1] == "--status" {
		showStatus()
		return
	}

	runOrchestrator()
}

func runOrchestrator() {
	ctx := context.Background()

	// Resolve config path relative to this example
	configPath := filepath.Join("examples", "orchestrator_embedded", "stages.yaml")

	fmt.Printf("Running orchestrator with config: %s\n", configPath)

	// Initialize orchestrator from config file
	orch, err := orchestrator.LoadFromFile(configPath)
	if err != nil {
		log.Fatalf("Failed to initialize orchestrator: %v", err)
	}

	// Execute all stages
	fmt.Println("Starting multi-stage execution...")
	err = orch.ExecuteStages(ctx, "", "") // Empty from/to means execute all stages
	if err != nil {
		log.Fatalf("Stage execution failed: %v", err)
	}

	fmt.Println("✅ All stages completed successfully!")
}

func runDryRun() {
	configPath := filepath.Join("examples", "orchestrator_embedded", "stages.yaml")
	fmt.Printf("Dry-run validation with config: %s\n", configPath)

	// For dry-run, we just validate the configuration
	_, err := orchestrator.LoadFromFile(configPath)
	if err != nil {
		log.Fatalf("Configuration validation failed: %v", err)
	}

	fmt.Println("✅ Configuration is valid")
	fmt.Println("📋 Execution would proceed with stages: infrastructure → database → services → configuration")
}

func showStatus() {
	configPath := filepath.Join("examples", "orchestrator_embedded", "stages.yaml")
	fmt.Printf("Status check with config: %s\n", configPath)

	orch, err := orchestrator.LoadFromFile(configPath)
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	fmt.Println("📊 Stages Status:")

	// Get stage results
	results := orch.GetStageResults()

	if len(results) == 0 {
		fmt.Println("No stages have been executed yet")
		return
	}

	for stageName, result := range results {
		status := "❌ Failed"
		if result.Success {
			status = "✅ Success"
		}

		fmt.Printf("  %s: %s", stageName, status)
		if result.Duration > 0 {
			fmt.Printf(" (took %v)", result.Duration)
		}
		fmt.Println()

		if result.Error != "" {
			fmt.Printf("    Error: %s\n", result.Error)
		}

		if len(result.ExtractedEnv) > 0 {
			fmt.Printf("    Extracted vars: %d\n", len(result.ExtractedEnv))
		}
	}
}
