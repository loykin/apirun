package main

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/spf13/viper"
	tc "github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

// CLI-level test: run up/down against PostgreSQL via Testcontainers and verify tables/records lifecycle.
func TestCmd_Postgres_UpDown_TablesAndRecords(t *testing.T) {
	ctx := context.Background()

	// Start PostgreSQL container
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
		// Skip if the environment cannot run containers
		t.Skipf("skipping Postgres cmd test: %v", err)
		return
	}
	defer func() { _ = pg.Terminate(ctx) }()

	host, err := pg.Host(ctx)
	if err != nil {
		t.Fatalf("container host: %v", err)
	}
	port, err := pg.MappedPort(ctx, "5432/tcp")
	if err != nil {
		t.Fatalf("container port: %v", err)
	}
	dsn := fmt.Sprintf("postgres://test:test@%s:%s/apimigrate_test?sslmode=disable", host, port.Port())

	// Test HTTP server to be called by migrations
	var delPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := r.URL.Path
		if strings.HasPrefix(p, "/create") {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(200)
			_, _ = w.Write([]byte(`{"id":"xyz"}`))
			return
		}
		if strings.HasPrefix(p, "/resource/") {
			delPath = p
			w.WriteHeader(200)
			_, _ = w.Write([]byte(`{"ok":true}`))
			return
		}
		w.WriteHeader(404)
	}))
	defer srv.Close()

	// Prepare temp dir with a simple up/down pair
	tdir := t.TempDir()
	mig := fmt.Sprintf(`---
up:
  name: create
  request:
    method: POST
    url: %s/create
  response:
    result_code: ["200"]
    env_from:
      rid: id

down:
  name: delete
  method: DELETE
  url: %s/resource/{{.rid}}
`, srv.URL, srv.URL)
	_ = writeFile(t, tdir, "001_create.yaml", mig)
	cfg := fmt.Sprintf(`---
migrate_dir: %s
store:
  save_response_body: false
  type: postgres
  postgres:
    dsn: %s
`, tdir, dsn)
	cfgPath := writeFile(t, tdir, "config.yaml", cfg)

	// Configure viper for CLI commands
	v := viper.GetViper()
	v.Set("config", cfgPath)
	v.Set("v", false)
	v.Set("to", 0)

	// Run up
	if err := upCmd.RunE(upCmd, nil); err != nil {
		t.Fatalf("up error: %v", err)
	}

	// Connect via sql to inspect tables/content
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatalf("sql open: %v", err)
	}
	defer func() { _ = db.Close() }()

	// Verify tables exist (including goose version table)
	mustHave := []string{"schema_migrations", "migration_runs", "stored_env", "apimigrate_goose_version"}
	for _, tbl := range mustHave {
		row := db.QueryRow(`SELECT 1 FROM information_schema.tables WHERE table_name = $1`, tbl)
		var one int
		if err := row.Scan(&one); err != nil {
			t.Fatalf("expected table %s to exist: %v", tbl, err)
		}
	}

	// stored_env should have the rid from env_from
	row := db.QueryRow(`SELECT COUNT(1) FROM stored_env WHERE version=1 AND name='rid' AND value='xyz'`)
	var cnt int
	if err := row.Scan(&cnt); err != nil {
		t.Fatalf("count stored_env: %v", err)
	}
	if cnt != 1 {
		t.Fatalf("expected 1 stored_env row, got %d", cnt)
	}

	// Run down
	if err := downCmd.RunE(downCmd, nil); err != nil {
		t.Fatalf("down error: %v", err)
	}
	if delPath != "/resource/xyz" {
		t.Fatalf("expected down to call /resource/xyz, got %s", delPath)
	}
	// stored_env should be cleaned
	row = db.QueryRow(`SELECT COUNT(1) FROM stored_env WHERE version=1`)
	cnt = -1
	if err := row.Scan(&cnt); err != nil {
		t.Fatalf("count stored_env after down: %v", err)
	}
	if cnt != 0 {
		t.Fatalf("expected 0 stored_env rows after down, got %d", cnt)
	}
}
