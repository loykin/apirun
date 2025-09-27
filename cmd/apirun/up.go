package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/loykin/apirun"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	ienv "github.com/loykin/apirun/pkg/env"
)

var upCmd = &cobra.Command{
	Use:   "up",
	Short: "Apply up migrations up to a target version (0 = all)",
	RunE: func(cmd *cobra.Command, args []string) error {
		v := viper.GetViper()
		configPath := v.GetString("config")
		if strings.TrimSpace(configPath) == "" {
			configPath = os.Getenv("APIMIGRATE_CONFIG")
		}
		dry := v.GetBool("dry_run")
		dryFrom := v.GetInt("dry_run_from")
		to := v.GetInt("to")
		noStore := v.GetBool("no_store")
		ctx := context.Background()
		be := ienv.New()
		baseEnv := be
		dir := ""
		saveResp := false
		var storeCfgFromDoc *apirun.StoreConfig
		if strings.TrimSpace(configPath) != "" {
			var doc ConfigDoc
			if err := doc.Load(configPath); err != nil {
				return err
			}
			mDir := strings.TrimSpace(doc.MigrateDir)
			if mDir == "" {
				// Fallback: use the directory of the config file if migrate_dir is not set
				mDir = filepath.Dir(configPath)
			}
			envFromCfg, err := doc.GetEnv()
			if err != nil {
				return err
			}
			if err := doWait(ctx, envFromCfg, doc.Wait, doc.Client); err != nil {
				return err
			}
			if err := doc.DecodeAuth(ctx, envFromCfg); err != nil {
				return err
			}
			// Override store disabled setting with CLI flag if provided
			if noStore {
				doc.Store.Disabled = true
			}
			// Build store options now; we'll pass them to Migrator below
			storeCfgFromDoc = doc.Store.ToStorOptions()
			saveBody := doc.Store.SaveResponseBody
			if mDir != "" {
				dir = mDir
			}
			// Always use env from config (may carry Auth even if Global is empty)
			baseEnv = envFromCfg
			saveResp = saveBody
		}
		if strings.TrimSpace(dir) == "" {
			dir = "./config/migration"
		}
		// Normalize to absolute path to avoid working-directory surprises
		if abs, err := filepath.Abs(dir); err == nil {
			dir = abs
		}
		m := apirun.Migrator{Env: baseEnv, Dir: dir, SaveResponseBody: saveResp, DryRun: dry, DryRunFrom: dryFrom}
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
		// Configure store via Migrator.StoreConfig (auto-connect inside MigrateUp)
		var scPtr *apirun.StoreConfig
		if strings.TrimSpace(configPath) != "" {
			// Reuse store config parsed earlier
			if storeCfgFromDoc != nil {
				scPtr = storeCfgFromDoc
			}
		}
		if scPtr == nil && !noStore {
			// default to sqlite under dir explicitly
			tmp := &apirun.StoreConfig{}
			tmp.Config.Driver = apirun.DriverSqlite
			tmp.Config.DriverConfig = &apirun.SqliteConfig{Path: filepath.Join(dir, apirun.StoreDBFileName)}
			scPtr = tmp
		}
		m.StoreConfig = scPtr
		_, err := m.MigrateUp(ctx, to)
		return err
	},
}
