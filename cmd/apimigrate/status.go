package main

import (
	"context"
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
		ctx := context.Background()
		dir := ""
		if strings.TrimSpace(configPath) != "" {
			if verbose {
				log.Printf("loading config from %s", configPath)
			}
			mDir, _, err := loadConfigAndAcquire(ctx, configPath, verbose)
			if err != nil {
				log.Printf("warning: failed to load config: %v", err)
			} else {
				if mDir != "" {
					dir = mDir
				}
			}
		}
		if strings.TrimSpace(dir) == "" {
			dir = "examples/migration"
		}
		dbPath := filepath.Join(dir, apimigrate.StoreDBFileName)
		st, err := apimigrate.OpenStore(dbPath)
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
