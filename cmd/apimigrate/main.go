package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

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
			mDir, envFromCfg, err := loadConfigAndAcquire(ctx, configPath, verbose)
			if err != nil {
				return err
			}
			if mDir != "" {
				dir = mDir
			}
			if len(envFromCfg.Global) > 0 {
				baseEnv = envFromCfg
			}
		}

		// If dir wasn't set by config, fall back to the conventional example path
		if strings.TrimSpace(dir) == "" {
			dir = "examples/migration"
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
			mDir, envFromCfg, err := loadConfigAndAcquire(ctx, configPath, verbose)
			if err != nil {
				return err
			}
			if mDir != "" {
				dir = mDir
			}
			if len(envFromCfg.Global) > 0 {
				baseEnv = envFromCfg
			}
		}
		if strings.TrimSpace(dir) == "" {
			dir = "examples/migration"
		}
		if verbose {
			log.Printf("up migrations in %s to %d", dir, to)
		}
		_, err := apimigrate.MigrateUp(ctx, dir, baseEnv, to)
		return err
	},
}

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
			mDir, envFromCfg, err := loadConfigAndAcquire(ctx, configPath, verbose)
			if err != nil {
				return err
			}
			if mDir != "" {
				dir = mDir
			}
			if len(envFromCfg.Global) > 0 {
				baseEnv = envFromCfg
			}
		}
		if strings.TrimSpace(dir) == "" {
			dir = "examples/migration"
		}
		if verbose {
			log.Printf("down migrations in %s to %d", dir, to)
		}
		_, err := apimigrate.MigrateDown(ctx, dir, baseEnv, to)
		return err
	},
}

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

func init() {
	// Defaults
	v := viper.GetViper()
	v.SetDefault("config", "")
	v.SetDefault("v", true)
	v.SetDefault("to", 0)

	// Environment variables support: APIMIGRATE_CONFIG, ...
	v.SetEnvPrefix("APIMIGRATE")
	v.AutomaticEnv()
	// Bind flags via Cobra and then bind to Viper
	rootCmd.PersistentFlags().String("config", v.GetString("config"), "path to a config yaml (like examples/keycloak-migration/config.yaml)")
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

func loadConfigAndAcquire(ctx context.Context, path string, verbose bool) (string, apimigrate.Env, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", apimigrate.Env{Global: map[string]string{}}, err
	}
	defer func() { _ = f.Close() }()
	dec := yaml.NewDecoder(f)
	migrateDir := ""
	base := apimigrate.Env{Global: map[string]string{}}
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
		// auth: type/config (preferred, single provider)
		processedProviders := false
		if strings.TrimSpace(doc.Auth.Type) != "" {
			ptype := doc.Auth.Type
			h, _, name, err := apimigrate.AcquireAuthByProviderSpec(ctx, ptype, doc.Auth.Config)
			if err != nil {
				return "", base, fmt.Errorf("auth type=%s: acquire failed: %w", ptype, err)
			}
			if verbose {
				log.Printf("auth %s: using header %s", strings.TrimSpace(name), h)
			}
			processedProviders = true
		}

		// auth providers (optional array)
		if !processedProviders && len(doc.Auth.Providers) > 0 {
			for i, item := range doc.Auth.Providers {
				ptypeRaw, ok := item["type"]
				if !ok {
					return "", base, fmt.Errorf("auth.providers[%d]: missing type", i)
				}
				ptype, _ := ptypeRaw.(string)
				h, _, name, err := apimigrate.AcquireAuthByProviderSpec(ctx, ptype, item)
				if err != nil {
					return "", base, fmt.Errorf("auth provider[%d] type=%s: acquire failed: %w", i, ptype, err)
				}
				if verbose {
					log.Printf("auth %s: using header %s", strings.TrimSpace(name), h)
				}
			}
			processedProviders = true
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
