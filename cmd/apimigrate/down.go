package main

import (
	"context"
	"log"
	"path/filepath"
	"strings"

	"github.com/loykin/apimigrate"
	"github.com/loykin/apimigrate/pkg/env"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var downCmd = &cobra.Command{
	Use:   "down",
	Short: "Rollback down to a target version",
	RunE: func(cmd *cobra.Command, args []string) error {
		v := viper.GetViper()
		configPath := v.GetString("config")
		verbose := v.GetBool("v")
		dry := v.GetBool("dry_run")
		dryFrom := v.GetInt("dry_run_from")
		to := v.GetInt("to")
		ctx := context.Background()
		be := env.New()
		baseEnv := &be
		dir := ""
		saveResp := false
		var storeCfgFromDoc *apimigrate.StoreConfig
		if strings.TrimSpace(configPath) != "" {
			if verbose {
				log.Printf("loading config from %s", configPath)
			}
			var doc ConfigDoc
			if err := doc.Load(configPath); err != nil {
				return err
			}
			mDir := strings.TrimSpace(doc.MigrateDir)
			if mDir == "" {
				// Fallback: use the directory of the config file if migrate_dir is not set
				mDir = filepath.Dir(configPath)
			}
			envFromCfg, err := doc.GetEnv(verbose)
			if err != nil {
				return err
			}
			if err := doWait(ctx, envFromCfg, doc.Wait, doc.Client, verbose); err != nil {
				return err
			}
			if err := doc.DecodeAuth(ctx, envFromCfg); err != nil {
				return err
			}
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
		if verbose {
			if dry {
				log.Printf("[dry-run] down migrations in %s to %d (from=%d)", dir, to, dryFrom)
			} else {
				log.Printf("down migrations in %s to %d", dir, to)
			}
		}
		m := apimigrate.Migrator{Env: *baseEnv, Dir: dir, SaveResponseBody: saveResp, DryRun: dry, DryRunFrom: dryFrom}
		// Set default render_body from config if provided
		if strings.TrimSpace(configPath) != "" {
			var doc ConfigDoc
			if err := doc.Load(configPath); err == nil {
				if doc.RenderBody != nil {
					m.RenderBodyDefault = doc.RenderBody
				}
			}
		}
		// Configure store via Migrator.StoreConfig (auto-connect inside MigrateDown)
		var scPtr *apimigrate.StoreConfig
		if strings.TrimSpace(configPath) != "" {
			if storeCfgFromDoc != nil {
				scPtr = storeCfgFromDoc
			}
		}
		if scPtr == nil {
			// default to sqlite under dir explicitly
			tmp := &apimigrate.StoreConfig{}
			tmp.Config.Driver = apimigrate.DriverSqlite
			tmp.Config.DriverConfig = &apimigrate.SqliteConfig{Path: filepath.Join(dir, apimigrate.StoreDBFileName)}
			scPtr = tmp
		}
		m.StoreConfig = scPtr
		_, err := m.MigrateDown(ctx, to)
		return err
	},
}
