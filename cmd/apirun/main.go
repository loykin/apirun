package main

import (
	"context"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var rootCmd = &cobra.Command{
	Use:   "apirun",
	Short: "Run API migrations defined in YAML files",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		runner := NewMigrationRunner(ctx)
		return runner.Run()
	},
}

func init() {
	// Defaults
	v := viper.GetViper()
	v.SetDefault("config", "./config/config.yaml")
	v.SetDefault("to", 0)
	v.SetDefault("dry_run", false)
	v.SetDefault("dry_run_from", 0)

	// Environment variables support: APIRUN_CONFIG, ...
	v.SetEnvPrefix("APIRUN")
	v.AutomaticEnv()
	// Bind flags via Cobra and then bind to Viper
	rootCmd.PersistentFlags().String("config", v.GetString("config"), "path to a config yaml (like examples/keycloak_migration/config.yaml)")
	upCmd.Flags().Int("to", v.GetInt("to"), "target version to migrate up to (0 = all)")
	upCmd.Flags().Bool("dry-run", v.GetBool("dry_run"), "simulate migrations without writing to the store")
	upCmd.Flags().Int("dry-run-from", v.GetInt("dry_run_from"), "version from which to start dry-run mode (0 = disabled)")
	downCmd.Flags().Int("to", v.GetInt("to"), "target version to migrate down to")
	downCmd.Flags().Bool("dry-run", v.GetBool("dry_run"), "simulate rollbacks without writing to the store")
	downCmd.Flags().Int("dry-run-from", v.GetInt("dry_run_from"), "version from which to start dry-run mode (0 = disabled)")

	_ = v.BindPFlag("config", rootCmd.PersistentFlags().Lookup("config"))
	_ = v.BindPFlag("to", upCmd.Flags().Lookup("to"))
	_ = v.BindPFlag("dry_run", upCmd.Flags().Lookup("dry-run"))
	_ = v.BindPFlag("dry_run_from", upCmd.Flags().Lookup("dry-run-from"))
	_ = v.BindPFlag("to", downCmd.Flags().Lookup("to"))
	_ = v.BindPFlag("dry_run", downCmd.Flags().Lookup("dry-run"))
	_ = v.BindPFlag("dry_run_from", downCmd.Flags().Lookup("dry-run-from"))

	rootCmd.AddCommand(upCmd)
	rootCmd.AddCommand(downCmd)
	rootCmd.AddCommand(statusCmd)
	rootCmd.AddCommand(createCmd)
	rootCmd.AddCommand(stagesCmd)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		exitHandler.LogFatalError(err, "command execution failed")
	}
}
