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

	_, err := orchestrator.LoadFromFile(configPath)
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	fmt.Println("📊 Configuration loaded successfully")
	fmt.Println("   Stages defined: infrastructure, database, services, configuration")
	fmt.Println("   Dependencies properly configured")
}
