package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/loykin/apimigrate"
	"github.com/loykin/apimigrate/internal/env"
	"gopkg.in/yaml.v3"
)

type SQLiteStoreConfig struct {
	Path string `mapstructure:"path" yaml:"path"`
}

type PostgresStoreConfig struct {
	DSN      string `mapstructure:"dsn" yaml:"dsn"`
	Host     string `mapstructure:"host" yaml:"host"`
	Port     int    `mapstructure:"port" yaml:"port"`
	User     string `mapstructure:"user" yaml:"user"`
	Password string `mapstructure:"password" yaml:"password"`
	DBName   string `mapstructure:"dbname" yaml:"dbname"`
	SSLMode  string `mapstructure:"sslmode" yaml:"sslmode"`
}

type AuthConfig struct {
	// Provider type key (e.g., "basic", "oauth2", "pocketbase")
	Type string `mapstructure:"type" yaml:"type"`
	// Logical name under which the acquired token will be stored
	Name string `mapstructure:"name" yaml:"name"`
	// Provider-specific configuration (rendered before acquisition)
	Config map[string]interface{} `mapstructure:"config" yaml:"config"`
	// Legacy: providers array inside the object (optional, alternative to single provider)
	Providers []map[string]interface{} `mapstructure:"providers" yaml:"providers"`
}

type EnvConfig struct {
	Name         string `mapstructure:"name" yaml:"name"`
	Value        string `mapstructure:"value" yaml:"value"`
	ValueFromEnv string `mapstructure:"valueFromEnv" yaml:"valueFromEnv"`
}

type StoreConfig struct {
	SaveResponseBody bool                `mapstructure:"save_response_body" yaml:"save_response_body"`
	Type             string              `mapstructure:"type" yaml:"type"`
	SQLite           SQLiteStoreConfig   `mapstructure:"sqlite" yaml:"sqlite"`
	Postgres         PostgresStoreConfig `mapstructure:"postgres" yaml:"postgres"`
	// Optional table name customization
	TablePrefix           string `mapstructure:"table_prefix" yaml:"table_prefix"`
	TableSchemaMigrations string `mapstructure:"table_schema_migrations" yaml:"table_schema_migrations"`
	TableMigrationRuns    string `mapstructure:"table_migration_runs" yaml:"table_migration_runs"`
	TableStoredEnv        string `mapstructure:"table_stored_env" yaml:"table_stored_env"`
}

func (c *StoreConfig) ToStorOptions() *apimigrate.StoreConfig {
	stType := strings.ToLower(strings.TrimSpace(c.Type))
	if stType == "" {
		return nil
	}

	// Derive table names: explicit values win; otherwise compute from prefix
	prefix := strings.TrimSpace(c.TablePrefix)
	sm := strings.TrimSpace(c.TableSchemaMigrations)
	mr := strings.TrimSpace(c.TableMigrationRuns)
	se := strings.TrimSpace(c.TableStoredEnv)
	if prefix != "" {
		if sm == "" {
			sm = prefix + "_schema_migrations"
		}
		if mr == "" {
			// use migration_log as agreed
			mr = prefix + "_migration_log"
		}
		if se == "" {
			se = prefix + "_stored_env"
		}
	}

	if stType == apimigrate.DriverPostgres {
		dsn := strings.TrimSpace(c.Postgres.DSN)
		if dsn == "" && strings.TrimSpace(c.Postgres.Host) != "" {
			port := c.Postgres.Port
			if port == 0 {
				port = 5432
			}
			ssl := strings.TrimSpace(c.Postgres.SSLMode)
			if ssl == "" {
				ssl = "disable"
			}
			dsn = fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=%s",
				strings.TrimSpace(c.Postgres.User), strings.TrimSpace(c.Postgres.Password),
				strings.TrimSpace(c.Postgres.Host), port, strings.TrimSpace(c.Postgres.DBName), ssl,
			)
		}
		pg := &apimigrate.PostgresConfig{DSN: dsn, Host: strings.TrimSpace(c.Postgres.Host), Port: c.Postgres.Port, User: strings.TrimSpace(c.Postgres.User), Password: strings.TrimSpace(c.Postgres.Password), DBName: strings.TrimSpace(c.Postgres.DBName), SSLMode: strings.TrimSpace(c.Postgres.SSLMode)}
		out := &apimigrate.StoreConfig{}
		out.Config.Driver = apimigrate.DriverPostgres
		out.Config.DriverConfig = pg
		// Table names customization (optional)
		out.Config.TableNames = apimigrate.TableNames{SchemaMigrations: sm, MigrationRuns: mr, StoredEnv: se}
		return out
	}
	// default to sqlite
	sqlite := &apimigrate.SqliteConfig{Path: strings.TrimSpace(c.SQLite.Path)}
	out := &apimigrate.StoreConfig{}
	out.Config.Driver = apimigrate.DriverSqlite
	out.Config.DriverConfig = sqlite
	out.Config.TableNames = apimigrate.TableNames{SchemaMigrations: sm, MigrationRuns: mr, StoredEnv: se}
	return out
}

type ClientConfig struct {
	// Explicit options only
	Insecure      bool   `mapstructure:"insecure"`
	MinTLSVersion string `mapstructure:"min_tls_version"`
	MaxTLSVersion string `mapstructure:"max_tls_version"`
}

type WaitConfig struct {
	URL      string `mapstructure:"url"`
	Method   string `mapstructure:"method"`
	Status   int    `mapstructure:"status"`
	Timeout  string `mapstructure:"timeout"`
	Interval string `mapstructure:"interval"`
}

type ConfigDoc struct {
	Auth       []AuthConfig `mapstructure:"auth"`
	MigrateDir string       `mapstructure:"migrate_dir"`
	Wait       WaitConfig   `mapstructure:"wait"`
	Env        []EnvConfig  `mapstructure:"env"`
	Store      StoreConfig  `mapstructure:"store"`
	Client     ClientConfig `mapstructure:"client"`
}

func (c *ConfigDoc) DecodeAuth(ctx context.Context, env *env.Env, verbose bool) error {
	for i, a := range c.Auth {
		pt := strings.TrimSpace(a.Type)
		if pt == "" {
			return fmt.Errorf("auth[%d]: missing type", i)
		}
		storedName := strings.TrimSpace(a.Name)
		if storedName == "" {
			return fmt.Errorf("auth[%d] type=%s: missing name (use auth[].name)", i, pt)
		}
		// Render templated values in the auth config using the base env
		renderedAny := apimigrate.RenderAnyTemplate(a.Config, *env)
		renderedCfg, _ := renderedAny.(map[string]interface{})
		_, err := apimigrate.AcquireAuthAndSetEnv(ctx, pt, storedName, apimigrate.NewAuthSpecFromMap(renderedCfg), env)
		if err != nil {
			return fmt.Errorf("auth[%d] type=%s name=%s: acquire failed: %w", i, pt, storedName, err)
		}
		if verbose {
			slog.Info("auth token acquired", "name", strings.TrimSpace(storedName))
		}
	}

	return nil
}

func (c *ConfigDoc) GetEnv(verbose bool) (apimigrate.Env, error) {
	base := apimigrate.NewEnv()

	// env (optional) - process before auth so templating can use it
	for _, kv := range c.Env {
		if kv.Name == "" {
			continue
		}
		val := kv.Value
		if val == "" && strings.TrimSpace(kv.ValueFromEnv) != "" {
			val = os.Getenv(kv.ValueFromEnv)
			if verbose && val == "" {
				slog.Error("warning: env %s requested from %s but variable is empty or not set", kv.Name, kv.ValueFromEnv)
			}
		}
		base.Global[kv.Name] = val
	}

	return base, nil
}

func (c *ConfigDoc) Load(path string) error {
	clean := filepath.Clean(path)
	// Ensure path points to a regular file to avoid opening directories/special files
	if info, statErr := os.Stat(clean); statErr != nil || !info.Mode().IsRegular() {
		if statErr != nil {
			return statErr
		}
		return fmt.Errorf("not a regular file: %s", clean)
	}
	// #nosec G304 -- config path is provided intentionally by the user/CI; cleaned and validated above
	f, err := os.Open(clean)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	dec := yaml.NewDecoder(f)
	return dec.Decode(c)
}
