package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/loykin/apimigrate/internal/httpc"
	"github.com/loykin/apimigrate/internal/migration"

	"github.com/go-viper/mapstructure/v2"
	"github.com/loykin/apimigrate"
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
		baseEnv := apimigrate.Env{Global: map[string]string{}}

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

		// If dir wasn't set by config, fall back to the conventional example path
		if strings.TrimSpace(dir) == "" {
			dir = "./migration"
		}
		if verbose {
			log.Printf("running migrations in %s (versioned, will be recorded)", dir)
		}
		// Use versioned executor so applied versions are persisted to the store
		vres, err := apimigrate.MigrateUp(ctx, dir, baseEnv, 0)
		if err != nil {
			if len(vres) > 0 && verbose {
				for _, vr := range vres {
					if vr != nil && vr.Result != nil {
						log.Printf("migration v%03d: status=%d env=%v", vr.Version, vr.Result.StatusCode, vr.Result.ExtractedEnv)
					}
				}
			}
			return err
		}

		if verbose {
			for _, vr := range vres {
				if vr != nil && vr.Result != nil {
					log.Printf("migration v%03d: status=%d env=%v", vr.Version, vr.Result.StatusCode, vr.Result.ExtractedEnv)
				}
			}
		}
		fmt.Println("migrations completed successfully")
		return nil
	},
}

func init() {
	// Defaults
	v := viper.GetViper()
	v.SetDefault("config", "./config/config.yaml")
	v.SetDefault("v", true)
	v.SetDefault("to", 0)

	// Environment variables support: APIMIGRATE_CONFIG, ...
	v.SetEnvPrefix("APIMIGRATE")
	v.AutomaticEnv()
	// Bind flags via Cobra and then bind to Viper
	rootCmd.PersistentFlags().String("config", v.GetString("config"), "path to a config yaml (like examples/keycloak_migration/config.yaml)")
	rootCmd.PersistentFlags().BoolP("v", "v", v.GetBool("v"), "verbose output")
	upCmd.Flags().Int("to", v.GetInt("to"), "target version to migrate up to (0 = all)")
	downCmd.Flags().Int("to", v.GetInt("to"), "target version to migrate down to")

	_ = v.BindPFlag("config", rootCmd.PersistentFlags().Lookup("config"))
	_ = v.BindPFlag("v", rootCmd.PersistentFlags().Lookup("v"))
	_ = v.BindPFlag("to", upCmd.Flags().Lookup("to"))
	_ = v.BindPFlag("to", downCmd.Flags().Lookup("to"))

	rootCmd.AddCommand(upCmd)
	rootCmd.AddCommand(downCmd)
	rootCmd.AddCommand(statusCmd)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		log.Printf("error: %v", err)
		os.Exit(1)
	}
}

func loadConfigAndAcquire(ctx context.Context, path string, verbose bool) (string, apimigrate.Env, bool, bool, string, string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", apimigrate.Env{Global: map[string]string{}}, false, false, "", "", err
	}
	defer func() { _ = f.Close() }()
	dec := yaml.NewDecoder(f)
	migrateDir := ""
	base := apimigrate.Env{Global: map[string]string{}}
	saveBody := false
	tlsInsecure := false
	tlsMin := ""
	tlsMax := ""
	for {
		var raw map[string]interface{}
		if err := dec.Decode(&raw); err != nil {
			if errors.Is(err, os.ErrClosed) {
				break
			}
			if err.Error() == "EOF" { // yaml v3 returns io.EOF but comparing string to avoid new import
				break
			}
			return "", base, false, false, "", "", err
		}
		// Decode with mapstructure into our strongly typed doc
		var doc ConfigDoc
		if err := mapstructure.Decode(raw, &doc); err != nil {
			return "", base, false, false, "", "", err
		}
		// read store options
		saveBody = doc.Store.SaveResponseBody
		// auth: new shape is an array of providers under doc.Auth
		if len(doc.Auth) > 0 {
			for i, a := range doc.Auth {
				pt := strings.TrimSpace(a.Type)
				if pt == "" {
					return "", base, false, false, "", "", fmt.Errorf("auth[%d]: missing type", i)
				}
				h, _, name, err := apimigrate.AcquireAuthByProviderSpec(ctx, pt, a.Config)
				if err != nil {
					return "", base, false, false, "", "", fmt.Errorf("auth[%d] type=%s: acquire failed: %w", i, pt, err)
				}
				if verbose {
					log.Printf("auth %s: using header %s", strings.TrimSpace(name), h)
				}
			}
		}
		// migrate_dir (optional)
		if strings.TrimSpace(doc.MigrateDir) != "" {
			// Use as provided: absolute paths unchanged; relative paths are relative to current working directory
			migrateDir = strings.TrimSpace(doc.MigrateDir)
		}
		// read client options
		tlsInsecure = doc.Client.Insecure
		tlsMin = strings.TrimSpace(doc.Client.MinTLSVersion)
		tlsMax = strings.TrimSpace(doc.Client.MaxTLSVersion)
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
	return migrateDir, base, saveBody, tlsInsecure, tlsMin, tlsMax, nil
}
