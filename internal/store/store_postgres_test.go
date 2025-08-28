package store

import (
	"context"
	"fmt"
	"testing"
	"time"

	tc "github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

// Integration test with PostgreSQL via testcontainers
func TestPostgresStore_BasicCRUD(t *testing.T) {
	ctx := context.Background()

	req := tc.ContainerRequest{
		Image:        "postgres:16",
		ExposedPorts: []string{"5432/tcp"},
		Env: map[string]string{
			"POSTGRES_USER":     "test",
			"POSTGRES_PASSWORD": "test",
			"POSTGRES_DB":       "apimigrate_test",
		},
		WaitingFor: wait.ForListeningPort("5432/tcp").WithStartupTimeout(120 * time.Second),
	}
	pg, err := tc.GenericContainer(ctx, tc.GenericContainerRequest{ContainerRequest: req, Started: true})
	if err != nil {
		// Skip on CI envs that cannot run containers, rather than failing whole suite
		t.Skipf("skipping Postgres container test: %v", err)
		return
	}
	defer func() { _ = pg.Terminate(ctx) }()

	host, err := pg.Host(ctx)
	if err != nil {
		_ = pg.Terminate(ctx)
		t.Fatalf("container host: %v", err)
	}
	port, err := pg.MappedPort(ctx, "5432/tcp")
	if err != nil {
		_ = pg.Terminate(ctx)
		t.Fatalf("container port: %v", err)
	}
	dsn := fmt.Sprintf("postgres://test:test@%s:%s/apimigrate_test?sslmode=disable", host, port.Port())

	st, err := OpenPostgres(dsn)
	if err != nil {
		_ = pg.Terminate(ctx)
		t.Fatalf("OpenPostgres: %v", err)
	}
	defer func() { _ = st.Close() }()

	// EnsureSchema is invoked inside OpenPostgres, but call again to assert idempotency
	if err := st.EnsureSchema(); err != nil {
		t.Fatalf("EnsureSchema: %v", err)
	}

	// Verify required tables exist (including goose version table)
	checks := []string{
		"schema_migrations",
		"migration_runs",
		"stored_env",
		"apimigrate_goose_version",
	}
	for _, tbl := range checks {
		row := st.DB.QueryRow(`SELECT 1 FROM information_schema.tables WHERE table_name = $1`, tbl)
		var one int
		if err := row.Scan(&one); err != nil {
			t.Fatalf("expected table %s to exist: %v", tbl, err)
		}
	}

	// Basic Apply/IsApplied/CurrentVersion/ListApplied
	for _, v := range []int{1, 3, 2} {
		if err := st.Apply(v); err != nil {
			t.Fatalf("Apply(%d): %v", v, err)
		}
	}
	ap, err := st.IsApplied(2)
	if err != nil || !ap {
		t.Fatalf("IsApplied(2) => %v,%v; want true,nil", ap, err)
	}
	cur, err := st.CurrentVersion()
	if err != nil {
		t.Fatalf("CurrentVersion: %v", err)
	}
	if cur != 3 {
		t.Fatalf("CurrentVersion=%d, want 3", cur)
	}

	// Stored env CRUD
	kv := map[string]string{"rid": "42", "user": "bob"}
	if err := st.InsertStoredEnv(2, kv); err != nil {
		t.Fatalf("InsertStoredEnv: %v", err)
	}
	m, err := st.LoadStoredEnv(2)
	if err != nil || m["rid"] != "42" || m["user"] != "bob" {
		t.Fatalf("LoadStoredEnv unexpected: m=%v err=%v", m, err)
	}
	if err := st.InsertStoredEnv(2, map[string]string{"rid": "99"}); err != nil {
		t.Fatalf("InsertStoredEnv replace: %v", err)
	}
	m, _ = st.LoadStoredEnv(2)
	if m["rid"] != "99" {
		t.Fatalf("expected rid=99 after replace, got %v", m["rid"])
	}

	// Record a run (with minimal fields) and then delete stored env
	if err := st.RecordRun(2, "up", 200, nil, map[string]string{"saved": "yes"}); err != nil {
		t.Fatalf("RecordRun: %v", err)
	}
	if err := st.DeleteStoredEnv(2); err != nil {
		t.Fatalf("DeleteStoredEnv: %v", err)
	}
	m, _ = st.LoadStoredEnv(2)
	if len(m) != 0 {
		t.Fatalf("expected empty after DeleteStoredEnv, got %v", m)
	}

	// Remove and verify
	if err := st.Remove(3); err != nil {
		t.Fatalf("Remove(3): %v", err)
	}
	ap, err = st.IsApplied(3)
	if err != nil || ap {
		t.Fatalf("IsApplied(3) after remove => %v,%v; want false,nil", ap, err)
	}

	// Small sleep to ensure async operations flushed (timestamps etc.)
	time.Sleep(100 * time.Millisecond)
}
