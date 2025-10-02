package main

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/loykin/apirun"
	"github.com/loykin/apirun/pkg/env"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var downCmd = &cobra.Command{
	Use:   "down",
	Short: "Rollback down to a target version",
	RunE: func(cmd *cobra.Command, args []string) error {
		v := viper.GetViper()
		configPath := v.GetString("config")
		dry := v.GetBool("dry_run")
		dryRunFrom := v.GetInt("dry_run_from")
		to := v.GetInt("to")
		ctx := context.Background()
		be := env.New()
		baseEnv := &be
		dir := ""
		saveResp := false
		var storeCfgFromDoc *apirun.StoreConfig
		if strings.TrimSpace(configPath) != "" {
			var doc ConfigDoc
			if err := doc.Load(configPath); err != nil {
				return fmt.Errorf("failed to load configuration file '%s': %w\nPlease verify the file exists and contains valid YAML", configPath, err)
			}
			mDir := strings.TrimSpace(doc.MigrateDir)
			if mDir == "" {
				// Fallback: use the directory of the config file if migrate_dir is not set
				mDir = filepath.Dir(configPath)
			}
			envFromCfg, err := doc.GetEnv()
			if err != nil {
				return fmt.Errorf("failed to process environment variables from config: %w", err)
			}
			if err := doWait(ctx, envFromCfg, doc.Wait, doc.Client); err != nil {
				return fmt.Errorf("dependency wait check failed: %w\nCheck that required services are running and accessible", err)
			}
			if err := doc.DecodeAuth(ctx, envFromCfg); err != nil {
				return fmt.Errorf("authentication setup failed: %w\nVerify auth configuration in config file", err)
			}
			// Store configuration is controlled via config file (store.disabled)
			storeCfgFromDoc = doc.Store.ToStorOptions()
			saveBody := doc.Store.SaveResponseBody
			if mDir != "" {
				dir = mDir
			}
			// Always use env from config (may carry Auth even if Global is empty)
			baseEnv = &envFromCfg
			saveResp = saveBody
		}
		if strings.TrimSpace(dir) == "" {
			dir = "./config/migration"
		}
		// Normalize to absolute path to avoid working-directory surprises
		if abs, err := filepath.Abs(dir); err == nil {
			dir = abs
		}
		m := apirun.Migrator{Env: *baseEnv, Dir: dir, SaveResponseBody: saveResp, DryRun: dry, DryRunFrom: dryRunFrom}
		// Set default render_body and delay from config if provided
		if strings.TrimSpace(configPath) != "" {
			var doc ConfigDoc
			if err := doc.Load(configPath); err == nil {
				if doc.RenderBody != nil {
					m.RenderBodyDefault = doc.RenderBody
				}
				if strings.TrimSpace(doc.DelayBetweenMigrations) != "" {
					if duration, err := time.ParseDuration(doc.DelayBetweenMigrations); err == nil {
						m.DelayBetweenMigrations = duration
					}
				}
			}
		}
		// Configure store via Migrator.StoreConfig (auto-connect inside MigrateDown)
		var scPtr *apirun.StoreConfig
		if strings.TrimSpace(configPath) != "" {
			if storeCfgFromDoc != nil {
				scPtr = storeCfgFromDoc
			}
		}
		if scPtr == nil {
			// default to sqlite under dir explicitly
			tmp := &apirun.StoreConfig{}
			tmp.Config.Driver = apirun.DriverSqlite
			tmp.Config.DriverConfig = &apirun.SqliteConfig{Path: filepath.Join(dir, apirun.StoreDBFileName)}
			scPtr = tmp
		}
		m.StoreConfig = scPtr
		_, err := m.MigrateDown(ctx, to)
		return err
	},
}
