package main

import (
	"context"

	"github.com/loykin/apirun/cmd/apirun/commands"
	"github.com/loykin/apirun/cmd/apirun/runner"
	"github.com/loykin/apirun/cmd/apirun/validation"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var rootCmd = &cobra.Command{
	Use:   "apirun",
	Short: "Run API migrations defined in YAML files",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		r := runner.NewMigrationRunner(ctx)
		return r.Run()
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
	commands.UpCmd.Flags().Int("to", v.GetInt("to"), "target version to migrate up to (0 = all)")
	commands.UpCmd.Flags().Bool("dry-run", v.GetBool("dry_run"), "simulate migrations without writing to the store")
	commands.UpCmd.Flags().Int("dry-run-from", v.GetInt("dry_run_from"), "version from which to start dry-run mode (0 = disabled)")
	commands.DownCmd.Flags().Int("to", v.GetInt("to"), "target version to migrate down to")
	commands.DownCmd.Flags().Bool("dry-run", v.GetBool("dry_run"), "simulate rollbacks without writing to the store")
	commands.DownCmd.Flags().Int("dry-run-from", v.GetInt("dry_run_from"), "version from which to start dry-run mode (0 = disabled)")

	_ = v.BindPFlag("config", rootCmd.PersistentFlags().Lookup("config"))
	_ = v.BindPFlag("to", commands.UpCmd.Flags().Lookup("to"))
	_ = v.BindPFlag("dry_run", commands.UpCmd.Flags().Lookup("dry-run"))
	_ = v.BindPFlag("dry_run_from", commands.UpCmd.Flags().Lookup("dry-run-from"))
	_ = v.BindPFlag("to", commands.DownCmd.Flags().Lookup("to"))
	_ = v.BindPFlag("dry_run", commands.DownCmd.Flags().Lookup("dry-run"))
	_ = v.BindPFlag("dry_run_from", commands.DownCmd.Flags().Lookup("dry-run-from"))

	rootCmd.AddCommand(commands.UpCmd)
	rootCmd.AddCommand(commands.DownCmd)
	rootCmd.AddCommand(commands.StatusCmd)
	rootCmd.AddCommand(commands.CreateCmd)
	rootCmd.AddCommand(commands.StagesCmd)
	rootCmd.AddCommand(validation.ValidateCmd)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		runner.DefaultHandler.LogFatalError(err, "command execution failed")
	}
}
