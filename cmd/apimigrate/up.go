package main

import (
	"context"
	"log"
	"strings"

	"github.com/loykin/apimigrate"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var upCmd = &cobra.Command{
	Use:   "up",
	Short: "Apply up migrations up to a target version (0 = all)",
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
			log.Printf("up migrations in %s to %d", dir, to)
		}
		m := apimigrate.Migrator{Env: baseEnv, Dir: dir, SaveResponseBody: saveResp}
		// Open store: from options when provided, otherwise default sqlite under dir
		var st *apimigrate.Store
		if strings.TrimSpace(configPath) != "" {
			_, _, _, _, _, _, storeOpts, _ := loadConfigAndAcquire(context.Background(), configPath, false)
			var err error
			st, err = apimigrate.OpenStoreFromOptions(dir, storeOpts)
			if err != nil {
				return err
			}
		} else {
			var err error
			st, err = apimigrate.OpenStoreFromOptions(dir, nil)
			if err != nil {
				return err
			}
		}
		defer func() { _ = st.Close() }()
		m.Store = *st
		_, err := m.MigrateUp(ctx, to)
		return err
	},
}
