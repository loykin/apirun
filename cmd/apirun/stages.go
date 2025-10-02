package main

import (
	"context"
	"fmt"

	"github.com/loykin/apirun/pkg/orchestrator"
	"github.com/spf13/cobra"
)

var stagesCmd = &cobra.Command{
	Use:   "stages",
	Short: "Manage multi-stage orchestration",
}

var stagesUpCmd = &cobra.Command{
	Use:   "up",
	Short: "Execute stages in dependency order",
	RunE: func(cmd *cobra.Command, args []string) error {
		return executeStagesCommand(cmd, true)
	},
}

var stagesDownCmd = &cobra.Command{
	Use:   "down",
	Short: "Execute down migrations for stages in reverse dependency order",
	RunE: func(cmd *cobra.Command, args []string) error {
		return executeStagesCommand(cmd, false)
	},
}

var stagesStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show status of stages and their dependencies",
	RunE: func(cmd *cobra.Command, args []string) error {
		configPath, _ := cmd.Flags().GetString("config")
		verbose, _ := cmd.Flags().GetBool("verbose")

		orch, err := orchestrator.LoadFromFile(configPath)
		if err != nil {
			return err
		}

		return showStagesStatus(orch, verbose)
	},
}

var stagesValidateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Validate stages configuration",
	RunE: func(cmd *cobra.Command, args []string) error {
		configPath, _ := cmd.Flags().GetString("config")

		_, err := orchestrator.LoadFromFile(configPath)
		if err != nil {
			fmt.Printf("❌ Validation failed: %v\n", err)
			return err
		}

		fmt.Println("✅ Configuration is valid")
		return nil
	},
}

func init() {
	// Add subcommands to stages
	stagesCmd.AddCommand(stagesUpCmd)
	stagesCmd.AddCommand(stagesDownCmd)
	stagesCmd.AddCommand(stagesStatusCmd)
	stagesCmd.AddCommand(stagesValidateCmd)

	// Add common flags to all stage commands
	addCommonStageFlags(stagesUpCmd)
	addCommonStageFlags(stagesDownCmd)
	addCommonStageFlags(stagesStatusCmd)
	addCommonStageFlags(stagesValidateCmd)

	// Add execution-specific flags to up/down commands
	addExecutionFlags(stagesUpCmd)
	addExecutionFlags(stagesDownCmd)

	// Add status-specific flags
	stagesStatusCmd.Flags().BoolP("verbose", "v", false, "Show detailed information")
}

func addCommonStageFlags(cmd *cobra.Command) {
	cmd.Flags().StringP("config", "c", "stages.yaml", "Path to stages configuration file")
}

func addExecutionFlags(cmd *cobra.Command) {
	cmd.Flags().String("from", "", "Start execution from this stage (inclusive)")
	cmd.Flags().String("to", "", "Execute up to this stage (inclusive)")
	cmd.Flags().String("stage", "", "Execute only this specific stage")
	cmd.Flags().Bool("dry-run", false, "Show execution plan without running")
}

// executeStagesCommand handles the common logic for up/down commands
func executeStagesCommand(cmd *cobra.Command, isUp bool) error {
	configPath, _ := cmd.Flags().GetString("config")
	fromStage, _ := cmd.Flags().GetString("from")
	toStage, _ := cmd.Flags().GetString("to")
	singleStage, _ := cmd.Flags().GetString("stage")
	dryRun, _ := cmd.Flags().GetBool("dry-run")

	// If single stage is specified, use it as both from and to
	if singleStage != "" {
		fromStage = singleStage
		toStage = singleStage
	}

	// Load orchestrator
	orch, err := orchestrator.LoadFromFile(configPath)
	if err != nil {
		return err // orchestrator.LoadFromFile already provides clear error messages
	}

	direction := "down"
	if isUp {
		direction = "up"
	}

	if dryRun {
		return showExecutionPlan(cmd, fromStage, toStage, direction)
	}

	// Execute stages
	ctx := context.Background()
	if isUp {
		err = orch.ExecuteStages(ctx, fromStage, toStage)
	} else {
		err = orch.ExecuteStagesDown(ctx, fromStage, toStage)
	}

	if err != nil {
		return err // Orchestrator methods already provide clear error messages
	}

	if isUp {
		fmt.Println("✅ All stages executed successfully")
	} else {
		fmt.Println("✅ All stages rolled back successfully")
	}
	return nil
}

func showExecutionPlan(cmd *cobra.Command, fromStage, toStage, direction string) error {
	configPath, _ := cmd.Flags().GetString("config")

	// Load orchestrator to get execution plan
	orch, err := orchestrator.LoadFromFile(configPath)
	if err != nil {
		return err
	}

	fmt.Printf("🔍 Execution plan (%s):\n", direction)

	if fromStage != "" {
		fmt.Printf("From stage: %s\n", fromStage)
	} else {
		fmt.Printf("From stage: <beginning>\n")
	}

	if toStage != "" {
		fmt.Printf("To stage: %s\n", toStage)
	} else {
		fmt.Printf("To stage: <end>\n")
	}

	// Get execution plan
	batches, err := orch.GetExecutionPlan(fromStage, toStage, direction)
	if err != nil {
		return fmt.Errorf("failed to get execution plan: %w", err)
	}

	if len(batches) == 0 {
		fmt.Println("\n📋 No stages to execute")
		return nil
	}

	fmt.Println("\n📋 Execution plan:")
	for i, batch := range batches {
		if len(batch) == 1 {
			fmt.Printf("  %d. %s\n", i+1, batch[0])
		} else {
			fmt.Printf("  %d. Parallel execution:\n", i+1)
			for _, stage := range batch {
				fmt.Printf("     • %s\n", stage)
			}
		}
	}

	fmt.Println("\n⚠️  This is a dry run - no changes will be made")

	return nil
}

func showStagesStatus(orch *orchestrator.Orchestrator, verbose bool) error {
	fmt.Println("📊 Stages Status:")

	// Get stage results
	results := orch.GetStageResults()

	if len(results) == 0 {
		fmt.Println("No stages have been executed yet")
		return nil
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

		if verbose && result.Error != "" {
			fmt.Printf("    Error: %s\n", result.Error)
		}

		if verbose && len(result.ExtractedEnv) > 0 {
			fmt.Printf("    Extracted vars: %d\n", len(result.ExtractedEnv))
			for k, v := range result.ExtractedEnv {
				fmt.Printf("      %s = %s\n", k, v)
			}
		}
	}

	return nil
}
