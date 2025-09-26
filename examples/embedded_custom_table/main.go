package main

import (
	"context"
	"fmt"
	"log"
	"path/filepath"

	"github.com/loykin/apirun"
	"github.com/loykin/apirun/pkg/env"
)

// This example demonstrates how to run apirun programmatically (embedded)
// while customizing the table/index names used by the store. This is useful
// if you want multiple, isolated sets of migration history in the same
// database file (or schema) by giving each set unique table names.
//
// Run from the repository root:
//
//	go run ./examples/embedded_custom_table
//
// It will create a SQLite database using custom table names and run
// the sample migration in this example directory.
func main() {
	// Directory containing versioned migrations for this example
	migrateDir := "examples/embedded_custom_table/migration"

	// Prepare context and customize store table names using a simple prefix approach
	// (equivalent to config: store.table_prefix: demo).
	ctx := context.Background()
	prefix := "demo"
	// Base env for templating (empty here)
	base := env.Env{Global: env.FromStringMap(map[string]string{})}

	// Configure store via StoreConfig: sqlite with custom table names
	storeConfig := apirun.StoreConfig{}
	storeConfig.Config.Driver = apirun.DriverSqlite
	storeConfig.Config.TableNames = apirun.TableNames{
		SchemaMigrations: prefix + "_schema_migrations",
		MigrationRuns:    prefix + "_migration_runs",
		StoredEnv:        prefix + "_stored_env",
	}
	// Let path default to <migrateDir>/apirun.db by leaving it empty here; Migrator's default handles it
	storeConfig.Config.DriverConfig = &apirun.SqliteConfig{Path: filepath.Join(migrateDir, apirun.StoreDBFileName)}
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
	fmt.Println("migrations completed successfully (custom table names, SQLite store)")
}
