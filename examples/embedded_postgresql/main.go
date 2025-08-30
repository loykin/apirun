package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/loykin/apimigrate"
)

// This example runs the versioned migrator programmatically while using
// a PostgreSQL-backed store for recording migration history.
//
// Prerequisites:
//   - A running PostgreSQL instance that you can connect to.
//   - Set PG_DSN env var or rely on the default
//     (postgres://postgres:postgres@localhost:5432/postgres?sslmode=disable).
//
// Run:
//
//	PG_DSN=postgres://user:pass@localhost:5432/postgres?sslmode=disable \
//	go run ./examples/embedded_postgresql
func main() {
	// Migration directory for this example
	migrateDir := "examples/embedded_postgresql/migration"

	// Read DSN from env or use a developer-friendly default
	dsn := os.Getenv("PG_DSN")
	if dsn == "" {
		dsn = "postgres://postgres:postgres@localhost:5432/postgres?sslmode=disable"
		log.Printf("PG_DSN not set, using default: %s", dsn)
	}

	// Prepare context with Postgres store options
	ctx := context.Background()
	storeOpts := &apimigrate.StoreOptions{Backend: "postgres", PostgresDSN: dsn}

	// Optional: saving response bodies can be toggled via Migrator.SaveResponseBody

	// Base environment (empty is fine for this example)
	base := apimigrate.Env{Global: map[string]string{}}

	// Run all migrations in the directory
	st, err := apimigrate.OpenStoreFromOptions(migrateDir, storeOpts)
	if err != nil {
		log.Fatalf("open postgres store failed: %v", err)
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
	fmt.Println("migrations completed successfully (PostgreSQL store)")
}
