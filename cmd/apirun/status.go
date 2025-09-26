package main

import (
	"fmt"
	"log"
	"path/filepath"
	"strings"

	"github.com/loykin/apirun"
	"github.com/loykin/apirun/pkg/status"
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

		dir := ""
		var storeCfg *apirun.StoreConfig
		var colorEnabled bool = false // Default to no color

		if strings.TrimSpace(configPath) != "" {
			var doc ConfigDoc
			if err := doc.Load(configPath); err != nil {
				log.Printf("warning: failed to load config: %v", err)
			} else {
				// Enable color from config if available
				if doc.Logging.Color != nil {
					colorEnabled = *doc.Logging.Color
				}

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
		st, err := apirun.OpenStoreFromOptions(dir, storeCfg)
		if err != nil {
			return err
		}
		defer func() { _ = st.Close() }()

		info, err := status.FromStore(st)
		if err != nil {
			return err
		}
		if statusHistory {
			fmt.Print(info.FormatColorizedWithLimit(true, statusHistoryLimit, statusHistoryAll, colorEnabled))
		} else {
			fmt.Print(info.FormatColorized(false, colorEnabled))
		}
		return nil
	},
}

func init() {
	statusCmd.Flags().BoolVar(&statusHistory, "history", false, "show migration run history as well")
	statusCmd.Flags().BoolVar(&statusHistoryAll, "history-all", false, "when used with --history, show all history entries (newest first)")
	statusCmd.Flags().IntVar(&statusHistoryLimit, "history-limit", 10, "when used with --history, show up to N latest entries (default 10)")
}
