package main

import (
	"context"
	"fmt"
	"log"

	"github.com/loykin/apimigrate"
)

// This example demonstrates how to run apimigrate programmatically (embedded)
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
	opts := &apimigrate.StoreOptions{
		Backend:               "sqlite",
		SQLitePath:            "", // empty -> defaults to <migrateDir>/apimigrate.db
		TableSchemaMigrations: prefix + "_schema_migrations",
		TableMigrationRuns:    prefix + "_migration_runs",
		TableStoredEnv:        prefix + "_stored_env",
	}
	// store options are now configured on the Migrator struct

	// Optional: whether to save response bodies into the runs table
	ctx = apimigrate.WithSaveResponseBody(ctx, false)

	// Base env for templating (empty here)
	base := apimigrate.Env{Global: map[string]string{}}

	// Apply all migrations in the directory
	st, err := apimigrate.OpenStoreFromOptions(migrateDir, opts)
	if err != nil {
		log.Fatalf("open store failed: %v", err)
	}
	defer func() { _ = st.Close() }()
	m := apimigrate.Migrator{Env: base, Dir: migrateDir, Store: *st}
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
