package main

import (
	"fmt"
	"strings"

	"github.com/loykin/apimigrate"
)

type SQLiteStoreConfig struct {
	Path string `mapstructure:"path"`
}

type PostgresStoreConfig struct {
	DSN      string `mapstructure:"dsn"`
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	User     string `mapstructure:"user"`
	Password string `mapstructure:"password"`
	DBName   string `mapstructure:"dbname"`
	SSLMode  string `mapstructure:"sslmode"`
}

type AuthConfig struct {
	// Provider type key (e.g., "basic", "oauth2", "pocketbase")
	Type string `mapstructure:"type"`
	// Logical name under which the acquired token will be stored
	Name string `mapstructure:"name"`
	// Provider-specific configuration (rendered before acquisition)
	Config map[string]interface{} `mapstructure:"config"`
	// Legacy: providers array inside the object (optional, alternative to single provider)
	Providers []map[string]interface{} `mapstructure:"providers"`
}

type EnvConfig struct {
	Name         string `mapstructure:"name"`
	Value        string `mapstructure:"value"`
	ValueFromEnv string `mapstructure:"valueFromEnv"`
}

type StoreConfig struct {
	SaveResponseBody bool                `mapstructure:"save_response_body"`
	Type             string              `mapstructure:"type"`
	SQLite           SQLiteStoreConfig   `mapstructure:"sqlite"`
	Postgres         PostgresStoreConfig `mapstructure:"postgres"`
	// Optional table name customization
	TablePrefix           string `mapstructure:"table_prefix"`
	TableSchemaMigrations string `mapstructure:"table_schema_migrations"`
	TableMigrationRuns    string `mapstructure:"table_migration_runs"`
	TableStoredEnv        string `mapstructure:"table_stored_env"`
}

func (c *StoreConfig) ToStorOptions() *apimigrate.StoreOptions {
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

	if stType == "postgres" || stType == "postgresql" || stType == "pg" {
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
		return &apimigrate.StoreOptions{
			Backend:               apimigrate.DriverPostgres,
			PostgresDSN:           dsn,
			TableSchemaMigrations: sm,
			TableMigrationRuns:    mr,
			TableStoredEnv:        se,
		}
	}
	// default to sqlite
	return &apimigrate.StoreOptions{
		Backend:               "sqlite",
		SQLitePath:            strings.TrimSpace(c.SQLite.Path),
		TableSchemaMigrations: sm,
		TableMigrationRuns:    mr,
		TableStoredEnv:        se,
	}
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
