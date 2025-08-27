package main

import (
	"context"
	"log"
	"strings"

	"github.com/loykin/apimigrate"
	"github.com/loykin/apimigrate/internal/httpc"
	"github.com/loykin/apimigrate/internal/migration"
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
		baseEnv := apimigrate.Env{Global: map[string]string{}}
		dir := ""
		if strings.TrimSpace(configPath) != "" {
			if verbose {
				log.Printf("loading config from %s", configPath)
			}
			mDir, envFromCfg, saveBody, tlsInsecure, tlsMin, tlsMax, err := loadConfigAndAcquire(ctx, configPath, verbose)
			if err != nil {
				return err
			}
			if mDir != "" {
				dir = mDir
			}
			if len(envFromCfg.Global) > 0 {
				baseEnv = envFromCfg
			}
			ctx = context.WithValue(ctx, migration.SaveResponseBodyKey, saveBody)
			if tlsInsecure {
				ctx = context.WithValue(ctx, httpc.CtxTLSInsecureKey, true)
			}
			if strings.TrimSpace(tlsMin) != "" {
				ctx = context.WithValue(ctx, httpc.CtxTLSMinVersionKey, strings.TrimSpace(tlsMin))
			}
			if strings.TrimSpace(tlsMax) != "" {
				ctx = context.WithValue(ctx, httpc.CtxTLSMaxVersionKey, strings.TrimSpace(tlsMax))
			}
		}
		if strings.TrimSpace(dir) == "" {
			dir = "./config/migration"
		}
		if verbose {
			log.Printf("up migrations in %s to %d", dir, to)
		}
		_, err := apimigrate.MigrateUp(ctx, dir, baseEnv, to)
		return err
	},
}
