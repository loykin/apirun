package main

import (
	"context"
	"log"
	"path/filepath"
	"strings"

	"github.com/loykin/apimigrate"
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
		to := v.GetInt("to")
		ctx := context.Background()
		baseEnv := apimigrate.NewEnv()
		dir := ""
		saveResp := false
		if strings.TrimSpace(configPath) != "" {
			if verbose {
				log.Printf("loading config from %s", configPath)
			}
			mDir, envFromCfg, saveBody, _, _, _, storeOpts, err := loadConfigAndAcquire(ctx, configPath, verbose)
			if err != nil {
				return err
			}
			if mDir != "" {
				dir = mDir
			}
			// Always use env from config (may carry Auth even if Global is empty)
			baseEnv = envFromCfg
			if storeOpts != nil {
				// store options are now applied via the Migrator struct
			}
			saveResp = saveBody
		}
		if strings.TrimSpace(dir) == "" {
			dir = "./config/migration"
		}
		if verbose {
			log.Printf("down migrations in %s to %d", dir, to)
		}
		m := apimigrate.Migrator{Env: baseEnv, Dir: dir, SaveResponseBody: saveResp}
		// Configure store via Migrator.StoreConfig (auto-connect inside MigrateDown)
		var storeCfg apimigrate.StoreConfig
		if strings.TrimSpace(configPath) != "" {
			_, _, _, _, _, _, storeOpts, _ := loadConfigAndAcquire(context.Background(), configPath, false)
			if storeOpts != nil && strings.ToLower(strings.TrimSpace(storeOpts.Backend)) == "postgres" {
				pg := apimigrate.PostgresConfig{DSN: strings.TrimSpace(storeOpts.PostgresDSN)}
				storeCfg = &pg
			} else {
				path := ""
				if storeOpts != nil {
					path = strings.TrimSpace(storeOpts.SQLitePath)
				}
				if path == "" {
					path = filepath.Join(dir, apimigrate.StoreDBFileName)
				}
				sqlite := apimigrate.SqliteConfig{Path: path}
				storeCfg = &sqlite
			}
		} else {
			sqlite := apimigrate.SqliteConfig{Path: filepath.Join(dir, apimigrate.StoreDBFileName)}
			storeCfg = &sqlite
		}
		m.StoreConfig = storeCfg
		_, err := m.MigrateDown(ctx, to)
		return err
	},
}
