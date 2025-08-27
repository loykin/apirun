package main

import (
	"context"
	"log"
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
		baseEnv := apimigrate.Env{Global: map[string]string{}}
		dir := ""
		if strings.TrimSpace(configPath) != "" {
			if verbose {
				log.Printf("loading config from %s", configPath)
			}
			mDir, envFromCfg, saveBody, err := loadConfigAndAcquire(ctx, configPath, verbose)
			if err != nil {
				return err
			}
			if mDir != "" {
				dir = mDir
			}
			if len(envFromCfg.Global) > 0 {
				baseEnv = envFromCfg
			}
			ctx = context.WithValue(ctx, "apimigrate.save_response_body", saveBody)
		}
		if strings.TrimSpace(dir) == "" {
			dir = "./config/migration"
		}
		if verbose {
			log.Printf("down migrations in %s to %d", dir, to)
		}
		_, err := apimigrate.MigrateDown(ctx, dir, baseEnv, to)
		return err
	},
}
