package apimigrate

import (
	"context"
	"fmt"
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

// Re-export commonly used types for public API

// Env is the environment layering structure used by migrations.
type Env = env.Env

// ExecResult is the result of a single task execution.
type ExecResult = task.ExecResult

// ExecWithVersion pairs an execution result with its version number.
type ExecWithVersion = imig.ExecWithVersion

// StoreOptions configures the persistence backend for migrations (sqlite or postgres).
// This is a type alias to the internal implementation for convenience in public APIs.
type StoreOptions = imig.StoreOptions

// Store is an alias to the internal store type.
type Store = store.Store

// StoreDBFileName is the default sqlite filename used for migration history.
const StoreDBFileName = store.DbFileName

// MigrateUp applies pending migrations up to targetVersion (0 = all).
func MigrateUp(ctx context.Context, dir string, base Env, targetVersion int) ([]*ExecWithVersion, error) {
	return imig.MigrateUp(ctx, dir, base, targetVersion)
}

// MigrateDown rolls back applied migrations down to targetVersion.
func MigrateDown(ctx context.Context, dir string, base Env, targetVersion int) ([]*ExecWithVersion, error) {
	return imig.MigrateDown(ctx, dir, base, targetVersion)
}

// AuthMethod Plugin-style provider interface and registration
type AuthMethod = auth.Method

type AuthFactory = auth.Factory

// RegisterAuthProvider exposes custom auth provider registration for library users.
func RegisterAuthProvider(typ string, f AuthFactory) { auth.Register(typ, f) }

// AcquireAuthByProviderSpecWithName acquires auth by provider type/spec but stores under the provided name,
// allowing callers to omit "name" inside spec and control the registry key explicitly.
// It returns only the acquired token value; header handling is decided by the caller (e.g., via env variables).
func AcquireAuthByProviderSpecWithName(ctx context.Context, typ string, name string, spec map[string]interface{}) (value string, err error) {
	v, err := auth.AcquireAndStoreWithName(ctx, typ, name, spec)
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

// AcquireAuthAndSetEnv acquires auth by provider type/spec, stores it under the given name,
// and automatically injects the acquired token into the provided base environment under
// the internal variable key "_auth_token". The token is stored as-is (trimmed), so if the
// provider returns a prefixed value like "Bearer ...", that full string is assigned.
// Returns the acquired token value.
func AcquireAuthAndSetEnv(ctx context.Context, typ string, name string, spec AuthSpec, base *Env) (string, error) {
	var mp map[string]interface{}
	if spec != nil {
		mp = spec.ToMap()
	}
	v, err := AcquireAuthByProviderSpecWithName(ctx, typ, name, mp)
	if err != nil {
		return "", err
	}
	if base != nil {
		if base.Global == nil {
			base.Global = map[string]string{}
		}
		base.Global[AuthTokenVar] = strings.TrimSpace(v)
	}
	return v, nil
}

// OpenStore opens (and initializes) the sqlite store at the given path.
func OpenStore(path string) (*Store, error) { return store.Open(path) }

// OpenStoreFromOptions opens a store based on StoreOptions.
// If opts is nil or backend is sqlite, opens sqlite at dir/StoreDBFileName or opts.SQLitePath when provided.
// For postgres, requires non-empty DSN.
func OpenStoreFromOptions(dir string, opts *StoreOptions) (*Store, error) {
	if opts == nil {
		path := filepath.Join(dir, StoreDBFileName)
		return store.Open(path)
	}
	switch strings.ToLower(strings.TrimSpace(opts.Backend)) {
	case "postgres", "postgresql", "pg":
		if strings.TrimSpace(opts.PostgresDSN) == "" {
			return nil, fmt.Errorf("store backend=postgres requires dsn")
		}
		// If any custom names provided, use the WithNames constructor
		if opts.TableSchemaMigrations != "" || opts.TableMigrationRuns != "" || opts.TableStoredEnv != "" || opts.IndexStoredEnvByVersion != "" {
			return store.OpenPostgresWithNames(opts.PostgresDSN, opts.TableSchemaMigrations, opts.TableMigrationRuns, opts.TableStoredEnv, opts.IndexStoredEnvByVersion)
		}
		return store.OpenPostgres(opts.PostgresDSN)
	default:
		path := strings.TrimSpace(opts.SQLitePath)
		if path == "" {
			path = filepath.Join(dir, StoreDBFileName)
		}
		if opts.TableSchemaMigrations != "" || opts.TableMigrationRuns != "" || opts.TableStoredEnv != "" || opts.IndexStoredEnvByVersion != "" {
			return store.OpenWithNames(path, opts.TableSchemaMigrations, opts.TableMigrationRuns, opts.TableStoredEnv, opts.IndexStoredEnvByVersion)
		}
		return store.Open(path)
	}
}

// WithSaveResponseBody returns a derived context that toggles saving response bodies
// alongside status codes in the migration history.
func WithSaveResponseBody(ctx context.Context, save bool) context.Context {
	return context.WithValue(ctx, imig.SaveResponseBodyKey, save)
}

// WithStoreOptions returns a derived context that carries StoreOptions instructing
// the migrator which backend/connection to use for the migration history store.
func WithStoreOptions(ctx context.Context, opts *StoreOptions) context.Context {
	return context.WithValue(ctx, imig.StoreOptionsKey, opts)
}

// WithTLSInsecure sets the TLS insecure flag for HTTP clients created by apimigrate.
func WithTLSInsecure(ctx context.Context, insecure bool) context.Context {
	return context.WithValue(ctx, httpc.CtxTLSInsecureKey, insecure)
}

// WithTLSMinVersion sets the minimum TLS version hint (e.g., "1.2" or "tls1.2").
func WithTLSMinVersion(ctx context.Context, min string) context.Context {
	return context.WithValue(ctx, httpc.CtxTLSMinVersionKey, strings.TrimSpace(min))
}

// WithTLSMaxVersion sets the maximum TLS version hint (e.g., "1.3" or "tls1.3").
func WithTLSMaxVersion(ctx context.Context, max string) context.Context {
	return context.WithValue(ctx, httpc.CtxTLSMaxVersionKey, strings.TrimSpace(max))
}

// NewHTTPClient returns a resty.Client configured from the provided context (TLS settings, etc.).
func NewHTTPClient(ctx context.Context) *resty.Client { return httpc.New(ctx) }

// RenderAnyTemplate exposes template rendering used for config/auth maps in the CLI.
func RenderAnyTemplate(v interface{}, base Env) interface{} { return util.RenderAnyTemplate(v, base) }
