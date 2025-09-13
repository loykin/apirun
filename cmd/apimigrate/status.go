package main

import (
	"fmt"
	"log"
	"path/filepath"
	"strings"

	"github.com/loykin/apimigrate"
	"github.com/loykin/apimigrate/pkg/status"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	statusHistory      bool
	statusHistoryAll   bool
	statusHistoryLimit int
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show current migration version, applied versions, and optionally history",
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

		info, err := status.FromStore(st)
		if err != nil {
			return err
		}
		if statusHistory {
			fmt.Print(info.FormatHumanWithLimit(true, statusHistoryLimit, statusHistoryAll))
		} else {
			fmt.Print(info.FormatHuman(false))
		}
		return nil
	},
}

func init() {
	statusCmd.Flags().BoolVar(&statusHistory, "history", false, "show migration run history as well")
	statusCmd.Flags().BoolVar(&statusHistoryAll, "history-all", false, "when used with --history, show all history entries (newest first)")
	statusCmd.Flags().IntVar(&statusHistoryLimit, "history-limit", 10, "when used with --history, show up to N latest entries (default 10)")
}
