package apimigrate

import (
	"context"
	"path/filepath"
	"strings"

	"github.com/go-resty/resty/v2"
	"github.com/loykin/apimigrate/internal/auth"
	"github.com/loykin/apimigrate/internal/env"
	"github.com/loykin/apimigrate/internal/httpc"
	imig "github.com/loykin/apimigrate/internal/migration"
	"github.com/loykin/apimigrate/internal/store"
	"github.com/loykin/apimigrate/internal/task"
	"github.com/loykin/apimigrate/internal/util"
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
	Env              Env
	StoreConfig      *StoreConfig
	SaveResponseBody bool
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

	im := imig.Migrator{Dir: m.Dir, Store: m.store, Env: m.Env, SaveResponseBody: m.SaveResponseBody}
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
	im := imig.Migrator{Dir: m.Dir, Store: m.store, Env: m.Env, SaveResponseBody: m.SaveResponseBody}
	return im.MigrateDown(ctx, targetVersion)
}

// Env is the environment layering structure used by migrations.
type Env = env.Env

// ExecResult is the result of a single task execution.
type ExecResult = task.ExecResult

// ExecWithVersion pairs an execution result with its version number.
type ExecWithVersion = imig.ExecWithVersion

// Store is an alias to the internal store type.
type Store = store.Store

// NewEnv returns a value Env with initialized internal maps, suitable for direct use
// in public APIs that expect Env by value.
func NewEnv() Env {
	p := env.New()
	return *p
}

// StoreDBFileName is the default sqlite filename used for migration history.
const StoreDBFileName = store.DbFileName

// AuthMethod Plugin-style provider interface and registration
type AuthMethod = auth.Method

type AuthFactory = auth.Factory

// RegisterAuthProvider exposes custom auth provider registration for library users.
func RegisterAuthProvider(typ string, f AuthFactory) { auth.Register(typ, f) }

// AcquireAuthByProviderSpecWithName acquires auth by provider type/spec but stores under the provided name,
// allowing callers to omit "name" inside spec and control the registry key explicitly.
// It returns only the acquired token value; header handling is decided by the caller (e.g., via env variables).
func AcquireAuthByProviderSpecWithName(ctx context.Context, typ string, spec map[string]interface{}) (value string, err error) {
	// name is kept for API compatibility but no longer used internally
	v, err := auth.AcquireAndStoreWithName(ctx, typ, spec)
	return v, err
}

// AuthSpec is a small interface allowing callers to pass strongly typed auth configs
// while keeping the internal registry based on map[string]interface{}.
// Implementations must return a map compatible with internal provider decoders.
type AuthSpec interface {
	ToMap() map[string]interface{}
}

// mapAuthSpec adapts a plain map into an AuthSpec.
type mapAuthSpec struct{ m map[string]interface{} }

func (s mapAuthSpec) ToMap() map[string]interface{} { return s.m }

// NewAuthSpecFromMap returns an AuthSpec backed by the provided map.
func NewAuthSpecFromMap(m map[string]interface{}) AuthSpec { return mapAuthSpec{m: m} }

// AcquireAuthAndSetEnv acquires auth by provider type/spec, and stores the acquired token
// into base.Auth[name] for template access via {{.auth.name}}. It returns the token value.
func AcquireAuthAndSetEnv(ctx context.Context, typ string, name string, spec AuthSpec, base *Env) (string, error) {
	var mp map[string]interface{}
	if spec != nil {
		mp = spec.ToMap()
	}
	v, err := AcquireAuthByProviderSpecWithName(ctx, typ, mp)
	if err != nil {
		return "", err
	}
	if base != nil {
		if base.Auth == nil {
			base.Auth = map[string]string{}
		}
		base.Auth[strings.TrimSpace(name)] = strings.TrimSpace(v)
	}
	return v, nil
}

// NewHTTPClient returns a resty.Client using default TLS settings (Min TLS 1.3).
// For custom TLS behavior, construct httpc.Httpc and call its New(ctx) method.
func NewHTTPClient(_ context.Context) *resty.Client { var h httpc.Httpc; return h.New() }

// RenderAnyTemplate exposes template rendering used for config/auth maps in the CLI.
func RenderAnyTemplate(v interface{}, base Env) interface{} { return util.RenderAnyTemplate(v, base) }

// OpenStoreFromOptions opens a store based on StoreConfig.
// If storeConfig is nil, opens sqlite at dir/StoreDBFileName.
// Otherwise, connects using the provided driver and driver config; for sqlite, missing path defaults to dir/StoreDBFileName.
func OpenStoreFromOptions(dir string, storeConfig *StoreConfig) (*Store, error) {
	// Default: sqlite under the provided directory
	if storeConfig == nil {
		path := filepath.Join(dir, StoreDBFileName)
		return store.Open(path)
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
		}
	}

	st := &store.Store{}
	st.TableName = cfg.TableNames
	if err := st.Connect(cfg); err != nil {
		return nil, err
	}
	return st, nil
}
