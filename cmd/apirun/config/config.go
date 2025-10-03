package config

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/loykin/apirun"
	iauth "github.com/loykin/apirun/internal/auth"
	"github.com/loykin/apirun/internal/store/postgresql"
	"github.com/loykin/apirun/internal/util"
	"github.com/loykin/apirun/pkg/env"
	"gopkg.in/yaml.v3"
)

type SQLiteStoreConfig struct {
	Path string `mapstructure:"path" yaml:"path"`
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
	Disabled         bool              `mapstructure:"disabled" yaml:"disabled" json:"disabled"`
	SaveResponseBody bool              `mapstructure:"save_response_body" yaml:"save_response_body"`
	Type             string            `mapstructure:"type" yaml:"type"`
	SQLite           SQLiteStoreConfig `mapstructure:"sqlite" yaml:"sqlite"`
	Postgres         postgresql.Config `mapstructure:"postgres" yaml:"postgres"`
	// Optional table name customization
	TablePrefix           string `mapstructure:"table_prefix" yaml:"table_prefix"`
	TableSchemaMigrations string `mapstructure:"table_schema_migrations" yaml:"table_schema_migrations"`
	TableMigrationRuns    string `mapstructure:"table_migration_runs" yaml:"table_migration_runs"`
	TableStoredEnv        string `mapstructure:"table_stored_env" yaml:"table_stored_env"`
}

func (c *StoreConfig) ToStorOptions() *apirun.StoreConfig {
	factory := NewStoreFactory()
	return factory.CreateStoreConfig(*c)
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
	// Ensure map exists
	if e.Auth == nil {
		e.Auth = env.Map{}
	}
	// Prepare lazy acquisition closures per auth name
	for i, a := range c.Auth {
		pt, ptOk := util.TrimEmptyCheck(a.Type)
		if !ptOk {
			return fmt.Errorf("auth[%d]: missing type", i)
		}
		storedName, nameOk := util.TrimEmptyCheck(a.Name)
		if !nameOk {
			return fmt.Errorf("auth[%d] type=%s: missing name (use auth[].name)", i, pt)
		}
		// Render templated values in the auth config using the base env
		renderedAny := apirun.RenderAnyTemplate(a.Config, e)
		renderedCfg, _ := renderedAny.(map[string]interface{})
		// Build struct-based config for later acquisition
		authCfg := &iauth.Auth{Type: pt, Name: storedName, Methods: iauth.NewAuthSpecFromMap(renderedCfg)}

		// Install lazy value using env.MakeLazy
		e.Auth[storedName] = e.MakeLazy(func(env *env.Env) (string, error) {
			// Use provided ctx if available; fall back to Background
			cctx := ctx
			if cctx == nil {
				cctx = context.Background()
			}
			return authCfg.Acquire(cctx, env)
		})
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
		if envVar, hasEnvVar := util.TrimEmptyCheck(kv.ValueFromEnv); val == "" && hasEnvVar {
			val = os.Getenv(envVar)
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
	level := util.TrimAndLower(c.Logging.Level)
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
	format := util.TrimAndLower(c.Logging.Format)

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
	levelStr := util.TrimWithDefault(util.TrimAndLower(c.Logging.Level), "info")

	logger.Info("logging configured",
		"level", levelStr,
		"format", format,
		"color", useColor,
		"mask_sensitive", maskingEnabled)

	return nil
}
