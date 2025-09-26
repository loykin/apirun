package main

import (
	"context"
	"fmt"
	"log"
	"path/filepath"

	"github.com/loykin/apirun"
	"github.com/loykin/apirun/pkg/env"
)

// This example runs the versioned migrator programmatically using
// the default SQLite-backed store (no extra configuration required).
//
// Run from the repository root:
//
//	go run ./examples/embedded_sqlite
//
// The migration history database (apirun.db) will be created under
// the example's migration directory.
func main() {
	// Directory containing versioned migrations for this example
	migrateDir := "examples/embedded_sqlite/migration"

	// Context can carry options; by default we use SQLite, so no store options are needed.
	ctx := context.Background()

	// Base environment (empty is fine for this example)
	base := env.Env{Global: env.FromStringMap(map[string]string{})}

	// Open the default SQLite store under the migration directory and attach it to the migrator
	st, err := apirun.OpenStoreFromOptions(migrateDir, nil)
	if err != nil {
		log.Fatalf("open store failed: %v", err)
	}
	defer func() { _ = st.Close() }()

	storeConfig := apirun.StoreConfig{}
	storeConfig.DriverConfig = &apirun.SqliteConfig{Path: filepath.Join(migrateDir, apirun.StoreDBFileName)}

	// Apply all migrations in the directory
	m := apirun.Migrator{Env: &base, Dir: migrateDir, StoreConfig: &storeConfig}
	vres, err := m.MigrateUp(ctx, 0)
	if err != nil {
		log.Fatalf("migrate up failed: %v", err)
	}
	for _, vr := range vres {
		if vr != nil && vr.Result != nil {
			fmt.Printf("v%03d: status=%d env=%v\n", vr.Version, vr.Result.StatusCode, vr.Result.ExtractedEnv)
		}
	}
	fmt.Println("migrations completed successfully (SQLite store)")
}
