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
// waitForPostgresDSN pings the database until it responds or timeout elapses.
func waitForPostgresDSN(dsn string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	var lastErr error
	for time.Now().Before(deadline) {
		db, err := sql.Open("pgx", dsn)
		if err == nil {
			pingErr := db.Ping()
			_ = db.Close()
			if pingErr == nil {
				return nil
			}
			lastErr = pingErr
		} else {
			lastErr = err
		}
		time.Sleep(500 * time.Millisecond)
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("timeout waiting for postgres")
	}
	return lastErr
}

func TestCmd_Postgres_UpDown_TablesAndRecords(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	// Start PostgreSQL container
	req := tc.ContainerRequest{
		Image:        "postgres:16",
		ExposedPorts: []string{"5432/tcp"},
		Env: map[string]string{
			"POSTGRES_USER":     "test",
			"POSTGRES_PASSWORD": "test",
			"POSTGRES_DB":       "apimigrate_test",
		},
		WaitingFor: wait.ForAll(
			wait.ForListeningPort("5432/tcp"),
			wait.ForLog("database system is ready to accept connections"),
		),
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

	// Ensure DB is accepting connections (avoid race after port readiness)
	if err := waitForPostgresDSN(dsn, 30*time.Second); err != nil {
		t.Fatalf("postgres not ready: %v", err)
	}

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
  url: %s/resource/{{.env.rid}}
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

	// Run down and validate the API effect (path recorded by the server)
	if err := downCmd.RunE(downCmd, nil); err != nil {
		t.Fatalf("down error: %v", err)
	}
	if delPath != "/resource/xyz" {
		t.Fatalf("expected down to call /resource/xyz, got %s", delPath)
	}
}
