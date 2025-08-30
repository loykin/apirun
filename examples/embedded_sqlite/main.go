package main

import (
	"context"
	"fmt"
	"log"

	"github.com/loykin/apimigrate"
)

// This example runs the versioned migrator programmatically using
// the default SQLite-backed store (no extra configuration required).
//
// Run from the repository root:
//
//	go run ./examples/embedded_sqlite
//
// The migration history database (apimigrate.db) will be created under
// the example's migration directory.
func main() {
	// Directory containing versioned migrations for this example
	migrateDir := "examples/embedded_sqlite/migration"

	// Context can carry options; by default we use SQLite, so no store options are needed.
	ctx := context.Background()
	// Optional: toggle saving response bodies (kept false for brevity)
	ctx = apimigrate.WithSaveResponseBody(ctx, false)

	// Base environment (empty is fine for this example)
	base := apimigrate.Env{Global: map[string]string{}}

	// Apply all migrations in the directory
	m := apimigrate.Migrator{Env: base, Dir: migrateDir}
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
