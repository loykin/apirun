package main

import (
	"fmt"
	"log"
	"path/filepath"
	"strings"

	"github.com/loykin/apimigrate"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show current migration version and applied versions",
	RunE: func(cmd *cobra.Command, args []string) error {
		v := viper.GetViper()
		configPath := v.GetString("config")
		verbose := v.GetBool("v")

		dir := ""
		var storeCfg *apimigrate.StoreConfig
		if strings.TrimSpace(configPath) != "" {
			if verbose {
				log.Printf("loading config from %s", configPath)
			}
			var doc ConfigDoc
			if err := doc.Load(configPath); err != nil {
				log.Printf("warning: failed to load config: %v", err)
			} else {
				mDir := strings.TrimSpace(doc.MigrateDir)
				if mDir == "" {
					// Fallback: use config file directory if migrate_dir not specified
					mDir = filepath.Dir(configPath)
				}
				tmpStoreCfg := doc.Store.ToStorOptions()
				if mDir != "" {
					dir = mDir
				}
				storeCfg = tmpStoreCfg
			}
		}
		if strings.TrimSpace(dir) == "" {
			dir = "./config/migration"
		}
		// centralized store opening
		st, err := apimigrate.OpenStoreFromOptions(dir, storeCfg)
		if err != nil {
			return err
		}
		defer func() { _ = st.Close() }()
		cur, err := st.CurrentVersion()
		if err != nil {
			return err
		}
		applied, err := st.ListApplied()
		if err != nil {
			return err
		}
		fmt.Printf("current: %d\napplied: %v\n", cur, applied)
		return nil
	},
}
