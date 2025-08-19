package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	mapstructure "github.com/go-viper/mapstructure/v2"
	"github.com/loykin/apimigrate/pkg/auth"
	"github.com/loykin/apimigrate/pkg/env"
	"github.com/loykin/apimigrate/pkg/migration"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
)

var rootCmd = &cobra.Command{
	Use:   "apimigrate",
	Short: "Run API migrations defined in YAML files",
	RunE: func(cmd *cobra.Command, args []string) error {
		// Bind and read configuration via Viper
		v := viper.GetViper()
		// Fetch values (viper already has defaults and env/flags bound)
		dir := ""
		configPath := v.GetString("config")
		verbose := v.GetBool("v")

		ctx := context.Background()
		baseEnv := env.Env{Global: map[string]string{}}

		if strings.TrimSpace(configPath) != "" {
			if verbose {
				log.Printf("loading config from %s", configPath)
			}
			mDir, envFromCfg, err := loadConfigAndAcquire(ctx, configPath, verbose)
			if err != nil {
				log.Printf("warning: failed to load config: %v", err)
			} else {
				if mDir != "" {
					dir = mDir
				}
				if len(envFromCfg.Global) > 0 {
					baseEnv = envFromCfg
				}
			}
		}

		// If dir wasn't set by config, fall back to the conventional example path
		if strings.TrimSpace(dir) == "" {
			dir = "examples/migration"
		}
		if verbose {
			log.Printf("running migrations in %s", dir)
		}
		// method and url are optional now; per-request overrides in YAML are expected
		results, err := migration.RunMigrationsWithEnv(ctx, dir, "", "", baseEnv)
		if err != nil {
			// Print any partial results first
			if len(results) > 0 && verbose {
				for i, r := range results {
					log.Printf("migration[%d]: status=%d env=%v", i, r.StatusCode, r.ExtractedEnv)
				}
			}
			return err
		}

		if verbose {
			for i, r := range results {
				log.Printf("migration[%d]: status=%d env=%v", i, r.StatusCode, r.ExtractedEnv)
			}
		}
		fmt.Println("migrations completed successfully")
		return nil
	},
}

func init() {
	// Defaults
	v := viper.GetViper()
	v.SetDefault("config", "")
	v.SetDefault("v", true)

	// Environment variables support: APIMIGRATE_CONFIG, ...
	v.SetEnvPrefix("APIMIGRATE")
	v.AutomaticEnv()
	// Bind flags via Cobra and then bind to Viper
	rootCmd.PersistentFlags().String("config", v.GetString("config"), "path to a config yaml (like examples/keycloak-migration/config.yaml)")
	rootCmd.PersistentFlags().BoolP("v", "v", v.GetBool("v"), "verbose output")

	_ = v.BindPFlag("config", rootCmd.PersistentFlags().Lookup("config"))
	_ = v.BindPFlag("v", rootCmd.PersistentFlags().Lookup("v"))
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		log.Printf("error: %v", err)
		os.Exit(1)
	}
}

func loadConfigAndAcquire(ctx context.Context, path string, verbose bool) (string, env.Env, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", env.Env{Global: map[string]string{}}, err
	}
	defer func() { _ = f.Close() }()
	dec := yaml.NewDecoder(f)
	migrateDir := ""
	base := env.Env{Global: map[string]string{}}
	for {
		var raw map[string]interface{}
		if err := dec.Decode(&raw); err != nil {
			if errors.Is(err, os.ErrClosed) {
				break
			}
			if err.Error() == "EOF" { // yaml v3 returns io.EOF but comparing string to avoid new import
				break
			}
			return "", base, err
		}
		// Decode with mapstructure into our strongly typed doc
		var doc ConfigDoc
		if err := mapstructure.Decode(raw, &doc); err != nil {
			return "", base, err
		}
		// auth provider (optional)
		pc := doc.Auth.Provider
		if strings.TrimSpace(pc.Name) != "" {
			cfg := auth.Config{Provider: pc}
			if h, _, err := auth.AcquireAndStore(ctx, cfg); err != nil {
				if verbose {
					log.Printf("auth provider %s: acquire failed: %v", pc.Name, err)
				}
			} else {
				if verbose {
					log.Printf("auth provider %s: using header %s", pc.Name, h)
				}
			}
		}
		// migrate_dir (optional)
		if strings.TrimSpace(doc.MigrateDir) != "" {
			md := doc.MigrateDir
			if !filepath.IsAbs(md) {
				md = filepath.Join(filepath.Dir(path), md)
			}
			migrateDir = md
		}
		// env (optional)
		for _, kv := range doc.Env {
			if kv.Name == "" {
				continue
			}
			val := kv.Value
			if val == "" && strings.TrimSpace(kv.ValueFromEnv) != "" {
				val = os.Getenv(kv.ValueFromEnv)
				if verbose && val == "" {
					log.Printf("warning: env %s requested from %s but variable is empty or not set", kv.Name, kv.ValueFromEnv)
				}
			}
			base.Global[kv.Name] = val
		}
	}
	// Do not treat lack of auth as an error to allow pure env/migrate_dir configs
	return migrateDir, base, nil
}
