package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/loykin/apirun"
	iauth "github.com/loykin/apirun/internal/auth"
	"github.com/loykin/apirun/pkg/env"
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

type LoggingConfig struct {
	Level         string `mapstructure:"level" yaml:"level"`                   // error, warn, info, debug
	Format        string `mapstructure:"format" yaml:"format"`                 // text, json, color
	MaskSensitive *bool  `mapstructure:"mask_sensitive" yaml:"mask_sensitive"` // enable/disable sensitive data masking
	Color         *bool  `mapstructure:"color" yaml:"color"`                   // enable/disable colorized output
}

type StoreConfig struct {
	Disabled         bool                `mapstructure:"disabled" yaml:"disabled" json:"disabled"`
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

// lazyVal is a Stringer that resolves once via proc when first printed
// used to install lazy .auth values into env.Auth Map without tying env to auth
type lazyVal struct {
	once sync.Once
	res  string
	err  error
	proc func() (string, error)
}

func (l *lazyVal) String() string {
	l.once.Do(func() {
		v, err := l.proc()
		if err != nil {
			l.err = err
			l.res = ""
			return
		}
		l.res = v
	})
	return l.res
}

func (c *StoreConfig) ToStorOptions() *apirun.StoreConfig {
	if c.Disabled {
		return nil
	}
	stType := strings.ToLower(strings.TrimSpace(c.Type))
	if stType == "" {
		return nil
	}

	tableNames := c.deriveTableNames()

	if stType == apirun.DriverPostgres {
		return c.createPostgresStoreConfig(tableNames)
	}

	return c.createSqliteStoreConfig(tableNames)
}

func (c *StoreConfig) deriveTableNames() apirun.TableNames {
	prefix := strings.TrimSpace(c.TablePrefix)
	sm := strings.TrimSpace(c.TableSchemaMigrations)
	mr := strings.TrimSpace(c.TableMigrationRuns)
	se := strings.TrimSpace(c.TableStoredEnv)

	if prefix != "" {
		if sm == "" {
			sm = prefix + "_schema_migrations"
		}
		if mr == "" {
			mr = prefix + "_migration_log"
		}
		if se == "" {
			se = prefix + "_stored_env"
		}
	}

	return apirun.TableNames{SchemaMigrations: sm, MigrationRuns: mr, StoredEnv: se}
}

func (c *StoreConfig) createPostgresStoreConfig(tableNames apirun.TableNames) *apirun.StoreConfig {
	pg := c.normalizePostgresConfig()
	pg.DSN = c.buildPostgresDSN()

	out := &apirun.StoreConfig{}
	out.Config.Driver = apirun.DriverPostgres
	out.Config.DriverConfig = pg
	out.Config.TableNames = tableNames
	return out
}

func (c *StoreConfig) normalizePostgresConfig() *apirun.PostgresConfig {
	return &apirun.PostgresConfig{
		Host:     strings.TrimSpace(c.Postgres.Host),
		Port:     c.Postgres.Port,
		User:     strings.TrimSpace(c.Postgres.User),
		Password: strings.TrimSpace(c.Postgres.Password),
		DBName:   strings.TrimSpace(c.Postgres.DBName),
		SSLMode:  strings.TrimSpace(c.Postgres.SSLMode),
	}
}

func (c *StoreConfig) buildPostgresDSN() string {
	dsn := strings.TrimSpace(c.Postgres.DSN)
	if dsn != "" {
		return dsn
	}

	pg := c.normalizePostgresConfig()
	if pg.Host == "" {
		return ""
	}

	port := pg.Port
	if port == 0 {
		port = 5432
	}

	ssl := pg.SSLMode
	if ssl == "" {
		ssl = "disable"
	}

	return fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=%s",
		pg.User, pg.Password, pg.Host, port, pg.DBName, ssl)
}

func (c *StoreConfig) createSqliteStoreConfig(tableNames apirun.TableNames) *apirun.StoreConfig {
	sqlite := &apirun.SqliteConfig{Path: strings.TrimSpace(c.SQLite.Path)}
	out := &apirun.StoreConfig{}
	out.Config.Driver = apirun.DriverSqlite
	out.Config.DriverConfig = sqlite
	out.Config.TableNames = tableNames
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
	Auth       []AuthConfig  `mapstructure:"auth" yaml:"auth"`
	MigrateDir string        `mapstructure:"migrate_dir" yaml:"migrate_dir"`
	Wait       WaitConfig    `mapstructure:"wait" yaml:"wait"`
	Env        []EnvConfig   `mapstructure:"env" yaml:"env"`
	Store      StoreConfig   `mapstructure:"store" yaml:"store"`
	Client     ClientConfig  `mapstructure:"client" yaml:"client"`
	Logging    LoggingConfig `mapstructure:"logging" yaml:"logging"`
	// Optional: control default rendering of request bodies with templates
	RenderBody *bool `mapstructure:"render_body" yaml:"render_body"`
	// DelayBetweenMigrations configures the delay between migration executions.
	// Can be specified as duration string (e.g., "500ms", "1s", "2m"). Defaults to "1s".
	DelayBetweenMigrations string `mapstructure:"delay_between_migrations" yaml:"delay_between_migrations"`
}

func (c *ConfigDoc) DecodeAuth(ctx context.Context, e *env.Env) error {
	// Prepare lazy acquisition closures per auth name
	procs := map[string]func() (string, error){}
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
		renderedAny := apirun.RenderAnyTemplate(a.Config, e)
		renderedCfg, _ := renderedAny.(map[string]interface{})
		// Build struct-based config for later acquisition
		authCfg := &iauth.Auth{Type: pt, Name: storedName, Methods: iauth.NewAuthSpecFromMap(renderedCfg)}
		procs[storedName] = func() (string, error) {
			// Use provided ctx if available; fall back to Background
			cctx := ctx
			if cctx == nil {
				cctx = context.Background()
			}
			return authCfg.Acquire(cctx, e)
		}
	}
	// Ensure map exists
	if e.Auth == nil {
		e.Auth = env.Map{}
	}
	// Install lazy values for each configured auth
	for name, proc := range procs {
		// preset values are not set here (DecodeAuth only wires lazies)
		e.Auth[name] = &lazyVal{proc: proc}
	}
	return nil
}

func (c *ConfigDoc) GetEnv() (*env.Env, error) {
	base := env.New()

	// env (optional) - process before auth so templating can use it
	for _, kv := range c.Env {
		if kv.Name == "" {
			continue
		}
		val := kv.Value
		if val == "" && strings.TrimSpace(kv.ValueFromEnv) != "" {
			val = os.Getenv(kv.ValueFromEnv)
			if val == "" {
				slog.Warn("env variable requested but empty or not set", "name", kv.Name, "env_var", kv.ValueFromEnv)
			}
		}
		_ = base.SetString("global", kv.Name, val)
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

func (c *ConfigDoc) parseLogLevel() (apirun.LogLevel, error) {
	level := strings.ToLower(strings.TrimSpace(c.Logging.Level))
	switch level {
	case "error":
		return apirun.LogLevelError, nil
	case "warn", "warning":
		return apirun.LogLevelWarn, nil
	case "info", "":
		return apirun.LogLevelInfo, nil
	case "debug":
		return apirun.LogLevelDebug, nil
	default:
		return apirun.LogLevelInfo, fmt.Errorf("invalid logging level: %s (valid: error, warn, info, debug)", c.Logging.Level)
	}
}

// SetupLogging configures the global logger based on config settings
func (c *ConfigDoc) SetupLogging() error {
	// Determine log level from config
	level, err := c.parseLogLevel()
	if err != nil {
		return err
	}

	// Determine format
	var logger *apirun.Logger
	format := strings.ToLower(strings.TrimSpace(c.Logging.Format))

	// Check if color is explicitly requested or auto-detect
	useColor := false
	if c.Logging.Color != nil {
		useColor = *c.Logging.Color
	} else if format == "color" || format == "colour" {
		useColor = true
	}

	switch format {
	case "json":
		logger = apirun.NewJSONLogger(level)
	case "color", "colour":
		logger = apirun.NewColorLogger(level)
	case "text", "":
		if useColor {
			logger = apirun.NewColorLogger(level)
		} else {
			// Default to text format
			logger = apirun.NewLogger(level)
		}
	default:
		return fmt.Errorf("invalid logging format: %s (valid: text, json, color)", c.Logging.Format)
	}

	// Configure masking
	maskingEnabled := true // Default to enabled
	if c.Logging.MaskSensitive != nil {
		maskingEnabled = *c.Logging.MaskSensitive
	}
	logger.EnableMasking(maskingEnabled)

	// Set as global logger
	apirun.SetDefaultLogger(logger)

	// Also set global masking state
	apirun.EnableMasking(maskingEnabled)

	// Log configuration info
	levelStr := strings.ToLower(strings.TrimSpace(c.Logging.Level))
	if levelStr == "" {
		levelStr = "info"
	}

	logger.Info("logging configured",
		"level", levelStr,
		"format", format,
		"color", useColor,
		"mask_sensitive", maskingEnabled)

	return nil
}
