package apimigrate

import (
	"context"
	"crypto/tls"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/loykin/apimigrate/internal/auth"
	"github.com/loykin/apimigrate/internal/common"
	imig "github.com/loykin/apimigrate/internal/migration"
	"github.com/loykin/apimigrate/internal/store"
	"github.com/loykin/apimigrate/internal/task"
	"github.com/loykin/apimigrate/internal/util"
	"github.com/loykin/apimigrate/pkg/env"
)

const DriverSqlite = store.DriverSqlite
const DriverPostgres = store.DriverPostgresql

type DriverConfig interface {
	ToMap() map[string]interface{}
}

type SqliteConfig = store.SqliteConfig
type PostgresConfig = store.PostgresConfig

type TableNames = store.TableNames
type StoreConfig struct {
	store.Config
}

// Migrator is the root struct-based API to run migrations programmatically.
// Users can create it, set Store and Env, then call its methods.
//
// Example:
//
//	m := apimigrate.Migrator{Dir: "./migrations", Store: apimigrate.StoreOptions{SQLitePath: "./state.db"}, Env: apimigrate.Env{Global: map[string]string{"k":"v"}}}
//	res, err := m.MigrateUp(ctx, 0)
//
// tokenStore is reserved for future enhancements and currently unused.
// It is kept to match the internal design and to allow future per-migrator token scoping.
// Exported fields mirror the user's configuration surface.
// #nosec G101 -- field name isn't a credential
type Migrator struct {
	Dir              string
	store            Store
	Env              *env.Env
	Auth             []auth.Auth
	StoreConfig      *StoreConfig
	SaveResponseBody bool
	// RenderBodyDefault controls default templating for RequestSpec bodies (nil = default true)
	RenderBodyDefault *bool
	// DryRun disables store mutations and simulates applied versions from DryRunFrom.
	DryRun bool
	// DryRunFrom indicates snapshot version already applied when DryRun is true (0 = from beginning).
	DryRunFrom int
	// TLSConfig applies to all HTTP requests executed during migrations
	TLSConfig *tls.Config
}

// MigrateUp applies pending migrations up to targetVersion (0 = all) using this Migrator's Store and Env.
func (m *Migrator) MigrateUp(ctx context.Context, targetVersion int) ([]*ExecWithVersion, error) {
	if m.StoreConfig != nil {
		// Support wrapper StoreConfig, direct store.Config, and direct driver configs
		cfg := m.StoreConfig
		if strings.TrimSpace(cfg.Driver) == "" {
			// Infer driver from DriverConfig type when not explicitly set
			switch cfg.DriverConfig.(type) {
			case *store.PostgresConfig:
				cfg.Driver = DriverPostgres
			default:
				cfg.Driver = DriverSqlite
			}
		}
		// For sqlite, ensure default file path under m.Dir when missing
		if strings.EqualFold(cfg.Driver, DriverSqlite) {
			if sc, ok := cfg.DriverConfig.(*store.SqliteConfig); ok {
				if strings.TrimSpace(sc.Path) == "" {
					sc.Path = filepath.Join(m.Dir, StoreDBFileName)
				}
			}
		}
		// Apply custom table names before connecting so EnsureSchema uses them
		m.store.TableName = cfg.TableNames
		if err := m.store.Connect(cfg.Config); err != nil {
			return nil, err
		}
	} else {
		// default sqlite under dir
		wrapper := store.Config{Driver: DriverSqlite,
			TableNames:   m.store.TableName,
			DriverConfig: &store.SqliteConfig{Path: filepath.Join(m.Dir, StoreDBFileName)}}
		if err := m.store.Connect(wrapper); err != nil {
			return nil, err
		}
	}

	im := imig.Migrator{Dir: m.Dir, Store: m.store, Env: m.Env, Auth: m.Auth, SaveResponseBody: m.SaveResponseBody, RenderBodyDefault: m.RenderBodyDefault, DryRun: m.DryRun, DryRunFrom: m.DryRunFrom, TLSConfig: m.TLSConfig}
	return im.MigrateUp(ctx, targetVersion)
}

// MigrateDown rolls back applied migrations down to targetVersion using this Migrator's Store and Env.
func (m *Migrator) MigrateDown(ctx context.Context, targetVersion int) ([]*ExecWithVersion, error) {
	if m.StoreConfig != nil {
		// Support wrapper StoreConfig, direct store.Config, and direct driver configs
		cfg := m.StoreConfig
		if strings.TrimSpace(cfg.Driver) == "" {
			// Infer driver from DriverConfig type when not explicitly set
			switch cfg.DriverConfig.(type) {
			case *store.PostgresConfig:
				cfg.Driver = DriverPostgres
			default:
				cfg.Driver = DriverSqlite
			}
		}
		// For sqlite, ensure default file path under m.Dir when missing
		if strings.EqualFold(cfg.Driver, DriverSqlite) {
			if sc, ok := cfg.DriverConfig.(*store.SqliteConfig); ok {
				if strings.TrimSpace(sc.Path) == "" {
					sc.Path = filepath.Join(m.Dir, StoreDBFileName)
				}
			}
		}
		// Apply custom table names before connecting so EnsureSchema uses them
		m.store.TableName = cfg.TableNames
		if err := m.store.Connect(cfg.Config); err != nil {
			return nil, err
		}
	} else if m.store.DB == nil {
		// default sqlite under dir
		wrapper := store.Config{Driver: DriverSqlite, DriverConfig: &store.SqliteConfig{Path: filepath.Join(m.Dir, StoreDBFileName)}}
		if err := m.store.Connect(wrapper); err != nil {
			return nil, err
		}
	}
	im := imig.Migrator{Dir: m.Dir, Store: m.store, Env: m.Env, Auth: m.Auth, SaveResponseBody: m.SaveResponseBody, RenderBodyDefault: m.RenderBodyDefault, DryRun: m.DryRun, DryRunFrom: m.DryRunFrom, TLSConfig: m.TLSConfig}
	return im.MigrateDown(ctx, targetVersion)
}

// Env is no longer re-exported here; use pkg/env.Env directly.

// ExecResult is the result of a single task execution.
type ExecResult = task.ExecResult

// ExecWithVersion pairs an execution result with its version number.
type ExecWithVersion = imig.ExecWithVersion

// Store is an alias to the internal store type.
type Store = store.Store

// RunHistory is a public representation of a migration run entry.
type RunHistory struct {
	ID         int
	Version    int
	Direction  string
	StatusCode int
	Failed     bool
	RanAt      string
	Body       *string
	Env        map[string]string
}

// ListRuns returns the migration run history for the provided store.
func ListRuns(st *Store) ([]RunHistory, error) {
	r, err := st.ListRuns()
	if err != nil {
		return nil, err
	}
	out := make([]RunHistory, 0, len(r))
	for _, it := range r {
		out = append(out, RunHistory{
			ID:         it.ID,
			Version:    it.Version,
			Direction:  it.Direction,
			StatusCode: it.StatusCode,
			Failed:     it.Failed,
			RanAt:      it.RanAt,
			Body:       it.Body,
			Env:        it.Env,
		})
	}
	return out, nil
}

// StoreDBFileName is the default sqlite filename used for migration history.
const StoreDBFileName = store.DbFileName

// AuthMethod Plugin-style provider interface and registration
type AuthMethod = auth.Method

type AuthFactory = auth.Factory

// Auth Expose struct-based auth configuration for external users.
type Auth = auth.Auth

type MethodConfig = auth.MethodConfig

func NewAuthSpecFromMap(m map[string]interface{}) MethodConfig { return auth.NewAuthSpecFromMap(m) }

// RegisterAuthProvider exposes custom auth provider registration for library users.
func RegisterAuthProvider(typ string, f AuthFactory) { auth.Register(typ, f) }

// RenderAnyTemplate exposes template rendering used for config/auth maps in the CLI.
func RenderAnyTemplate(v interface{}, base *env.Env) interface{} {
	return util.RenderAnyTemplate(v, base)
}

// OpenStoreFromOptions opens a store based on StoreConfig.
// If storeConfig is nil, opens sqlite at dir/StoreDBFileName.
// Otherwise, connects using the provided driver and driver config; for sqlite, missing path defaults to dir/StoreDBFileName.
func OpenStoreFromOptions(dir string, storeConfig *StoreConfig) (*Store, error) {
	// Default: sqlite under the provided directory
	if storeConfig == nil {
		storeConfig = &StoreConfig{}
		storeConfig.Config.Driver = DriverSqlite
		storeConfig.Config.DriverConfig = &SqliteConfig{Path: filepath.Join(dir, StoreDBFileName)}
	}

	cfg := storeConfig.Config
	// If driver not set, infer from driver config type (defaults to sqlite)
	drv := strings.ToLower(strings.TrimSpace(cfg.Driver))
	if drv == "" {
		switch cfg.DriverConfig.(type) {
		case *store.PostgresConfig:
			drv = DriverPostgres
		default:
			drv = DriverSqlite
		}
		cfg.Driver = drv
	}

	// For sqlite, ensure a default path when empty
	if cfg.Driver == DriverSqlite {
		if sc, ok := cfg.DriverConfig.(*store.SqliteConfig); ok {
			if strings.TrimSpace(sc.Path) == "" {
				sc.Path = filepath.Join(dir, StoreDBFileName)
			}

			if err := os.MkdirAll(filepath.Dir(sc.Path), 0o750); err != nil {
				return nil, fmt.Errorf("failed to create sqlite dir: %w", err)
			}
		}
	}

	st := &store.Store{}
	st.TableName = cfg.TableNames
	if err := st.Connect(cfg); err != nil {
		return nil, err
	}
	return st, nil
}

// Logging API - Public interface for structured logging

// LogLevel represents logging verbosity levels
type LogLevel int

const (
	LogLevelError LogLevel = iota
	LogLevelWarn
	LogLevelInfo
	LogLevelDebug
)

// Logger provides structured logging interface
type Logger struct {
	*slog.Logger
}

// NewLogger creates a new structured logger with the specified level
func NewLogger(level LogLevel) *Logger {
	var internalLevel common.LogLevel
	switch level {
	case LogLevelError:
		internalLevel = common.LogLevelError
	case LogLevelWarn:
		internalLevel = common.LogLevelWarn
	case LogLevelInfo:
		internalLevel = common.LogLevelInfo
	case LogLevelDebug:
		internalLevel = common.LogLevelDebug
	default:
		internalLevel = common.LogLevelInfo
	}

	internalLogger := common.NewLogger(internalLevel)
	return &Logger{Logger: internalLogger.Logger}
}

// NewJSONLogger creates a structured logger with JSON output
func NewJSONLogger(level LogLevel) *Logger {
	var internalLevel common.LogLevel
	switch level {
	case LogLevelError:
		internalLevel = common.LogLevelError
	case LogLevelWarn:
		internalLevel = common.LogLevelWarn
	case LogLevelInfo:
		internalLevel = common.LogLevelInfo
	case LogLevelDebug:
		internalLevel = common.LogLevelDebug
	default:
		internalLevel = common.LogLevelInfo
	}

	internalLogger := common.NewJSONLogger(internalLevel)
	return &Logger{Logger: internalLogger.Logger}
}

// SetDefaultLogger sets the global default logger for apimigrate
func SetDefaultLogger(logger *Logger) {
	internalLogger := &common.Logger{Logger: logger.Logger}
	common.SetDefaultLogger(internalLogger)
}

// GetLogger returns the default logger
func GetLogger() *Logger {
	internalLogger := common.GetLogger()
	return &Logger{Logger: internalLogger.Logger}
}
