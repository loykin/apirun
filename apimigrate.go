package apimigrate

import (
	"context"

	"github.com/loykin/apimigrate/internal/auth"
	"github.com/loykin/apimigrate/internal/env"
	imig "github.com/loykin/apimigrate/internal/migration"
	"github.com/loykin/apimigrate/internal/store"
	"github.com/loykin/apimigrate/internal/task"
)

// Re-export commonly used types for public API

// Env is the environment layering structure used by migrations.
type Env = env.Env

// ExecResult is the result of a single task execution.
type ExecResult = task.ExecResult

// ExecWithVersion pairs an execution result with its version number.
type ExecWithVersion = imig.ExecWithVersion

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

func AcquireAuthByProviderSpec(ctx context.Context, typ string, spec map[string]interface{}) (header, value, name string, err error) {
	return auth.AcquireAndStoreFromMap(ctx, typ, spec)
}

// Store is an alias to the internal store type.
type Store = store.Store

// StoreDBFileName is the default sqlite filename used for migration history.
const StoreDBFileName = store.StoreDBFileName

// OpenStore opens (and initializes) the sqlite store at the given path.
func OpenStore(path string) (*Store, error) { return store.Open(path) }
