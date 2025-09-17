package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/loykin/apimigrate"
	"github.com/loykin/apimigrate/internal/common"
	ienv "github.com/loykin/apimigrate/pkg/env"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var rootCmd = &cobra.Command{
	Use:   "apimigrate",
	Short: "Run API migrations defined in YAML files",
	RunE: func(cmd *cobra.Command, args []string) error {
		// Bind and read configuration via Viper
		v := viper.GetViper()
		// Fetch values (viper already has defaults and env/flags bound)
		dir := ""
		configPath := v.GetString("config")

		// Initialize basic logger - will be reconfigured from config file
		logger := common.NewLogger(common.LogLevelInfo)
		common.SetDefaultLogger(logger)

		logger.Info("starting apimigrate", "config_path", configPath)

		ctx := context.Background()
		be := ienv.New()
		baseEnv := be
		// store options are handled by default sqlite behavior in Migrator when not explicitly set
		saveResp := false
		// optional TLS settings from config
		var clientTLS *tls.Config

		if strings.TrimSpace(configPath) != "" {
			logger.Debug("loading configuration file", "path", configPath)
			var doc ConfigDoc
			if err := doc.Load(configPath); err != nil {
				return err
			}

			// Setup logging from config (this may override the initial logger)
			if err := doc.SetupLogging(); err != nil {
				return fmt.Errorf("failed to setup logging from config: %w", err)
			}

			// Get updated logger after potential reconfiguration
			logger = common.GetLogger().WithComponent("main")

			mDir := strings.TrimSpace(doc.MigrateDir)
			logger.Debug("processing configuration", "migrate_dir", mDir)
			envFromCfg, err := doc.GetEnv()
			if err != nil {
				logger.Error("failed to get environment from config", "error", err)
				return err
			}
			if err := doWait(ctx, envFromCfg, doc.Wait, doc.Client); err != nil {
				logger.Error("wait check failed", "error", err)
				return err
			}
			if err := doc.DecodeAuth(ctx, envFromCfg); err != nil {
				logger.Error("failed to decode authentication", "error", err)
				return err
			}
			_ = doc.Store.ToStorOptions()
			saveBody := doc.Store.SaveResponseBody
			if mDir != "" {
				dir = mDir
			}
			// Always use env from config (may carry Auth even if Global is empty)
			baseEnv = envFromCfg
			saveResp = saveBody
			// Build TLS config for client based on doc.Client
			minV := uint16(0)
			maxV := uint16(0)
			switch strings.TrimSpace(strings.ToLower(doc.Client.MinTLSVersion)) {
			case "1.0", "10", "tls1.0", "tls10":
				minV = tls.VersionTLS10
			case "1.1", "11", "tls1.1", "tls11":
				minV = tls.VersionTLS11
			case "1.2", "12", "tls1.2", "tls12":
				minV = tls.VersionTLS12
			case "1.3", "13", "tls1.3", "tls13":
				minV = tls.VersionTLS13
			}
			switch strings.TrimSpace(strings.ToLower(doc.Client.MaxTLSVersion)) {
			case "1.0", "10", "tls1.0", "tls10":
				maxV = tls.VersionTLS10
			case "1.1", "11", "tls1.1", "tls11":
				maxV = tls.VersionTLS11
			case "1.2", "12", "tls1.2", "tls12":
				maxV = tls.VersionTLS12
			case "1.3", "13", "tls1.3", "tls13":
				maxV = tls.VersionTLS13
			}
			cfg := &tls.Config{MinVersion: minV, MaxVersion: maxV}
			if doc.Client.Insecure {
				cfg.InsecureSkipVerify = true
			}
			clientTLS = cfg
			logger.Debug("TLS configuration applied", "insecure", doc.Client.Insecure, "min_version", minV, "max_version", maxV)
		} else {
			logger.Debug("using default configuration (no config file specified)")
		}

		// If dir wasn't set by config, fall back to the conventional example path
		if strings.TrimSpace(dir) == "" {
			dir = "./migration"
		}
		logger.Debug("running migrations", "dir", dir, "versioned", true)
		// Use versioned executor so applied versions are persisted to the store
		m := apimigrate.Migrator{Env: baseEnv, Dir: dir, SaveResponseBody: saveResp, TLSConfig: clientTLS}
		// Use default store behavior (sqlite under dir) unless programmatic StoreConfig is provided elsewhere
		vres, err := m.MigrateUp(ctx, 0)
		if err != nil {
			if len(vres) > 0 {
				for _, vr := range vres {
					if vr != nil && vr.Result != nil {
						logger.Debug("migration result", "version", vr.Version, "status_code", vr.Result.StatusCode, "env", vr.Result.ExtractedEnv)
					}
				}
			}
			return err
		}

		for _, vr := range vres {
			if vr != nil && vr.Result != nil {
				logger.Debug("migration result", "version", vr.Version, "status_code", vr.Result.StatusCode, "env", vr.Result.ExtractedEnv)
			}
		}
		fmt.Println("migrations completed successfully")
		return nil
	},
}

func init() {
	// Defaults
	v := viper.GetViper()
	v.SetDefault("config", "./config/config.yaml")
	v.SetDefault("to", 0)
	v.SetDefault("dry_run", false)
	v.SetDefault("dry_run_from", 0)

	// Environment variables support: APIMIGRATE_CONFIG, ...
	v.SetEnvPrefix("APIMIGRATE")
	v.AutomaticEnv()
	// Bind flags via Cobra and then bind to Viper
	rootCmd.PersistentFlags().String("config", v.GetString("config"), "path to a config yaml (like examples/keycloak_migration/config.yaml)")
	upCmd.Flags().Int("to", v.GetInt("to"), "target version to migrate up to (0 = all)")
	upCmd.Flags().Bool("dry-run", v.GetBool("dry_run"), "simulate migrations without writing to the store")
	upCmd.Flags().Int("dry-run-from", v.GetInt("dry_run_from"), "simulate as if versions up to N are already applied")
	downCmd.Flags().Int("to", v.GetInt("to"), "target version to migrate down to")
	downCmd.Flags().Bool("dry-run", v.GetBool("dry_run"), "simulate rollbacks without writing to the store")
	downCmd.Flags().Int("dry-run-from", v.GetInt("dry_run_from"), "simulate as if versions up to N are already applied")

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
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		log.Printf("error: %v", err)
		os.Exit(1)
	}
}
