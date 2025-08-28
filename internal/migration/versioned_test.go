package migration

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/loykin/apimigrate/internal/env"
	"github.com/loykin/apimigrate/internal/store"
)

// Ensures that MigrateDown calls Task.DownExecute (delegation) and rolls back in reverse order.
func TestMigrateDown_DelegatesAndReverseOrder(t *testing.T) {
	var received []string
	// Server records X-Seq header for down requests
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// For both up and down we just return 200; we only record when header present
		if v := r.Header.Get("X-Seq"); v != "" {
			received = append(received, v)
		}
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	dir := t.TempDir()
	// Create two migrations that have both up and down sections.
	// Up sections will be used to apply; Down sections will be used to rollback.
	mig1 := "up:\n  name: one\n  env: { }\n  request:\n    method: POST\n    url: " + srv.URL + "\n  response:\n    result_code: [\"200\"]\n" +
		"down:\n  name: one-down\n  env: { }\n  method: DELETE\n  url: " + srv.URL + "\n  headers:\n    - { name: X-Seq, value: \"1\" }\n"
	mig2 := "up:\n  name: two\n  env: { }\n  request:\n    method: POST\n    url: " + srv.URL + "\n  response:\n    result_code: [\"200\"]\n" +
		"down:\n  name: two-down\n  env: { }\n  method: DELETE\n  url: " + srv.URL + "\n  headers:\n    - { name: X-Seq, value: \"2\" }\n"
	if err := os.WriteFile(filepath.Join(dir, "001_first.yaml"), []byte(mig1), 0o600); err != nil {
		t.Fatalf("write mig1: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "002_second.yaml"), []byte(mig2), 0o600); err != nil {
		t.Fatalf("write mig2: %v", err)
	}

	ctx := context.Background()
	base := env.Env{Global: map[string]string{}}
	// Apply both
	if _, err := MigrateUp(ctx, dir, base, 0); err != nil {
		t.Fatalf("migrate up failed: %v", err)
	}
	// Rollback to 0 (all downs), expect order 2 then 1
	res, err := MigrateDown(ctx, dir, base, 0)
	if err != nil {
		t.Fatalf("migrate down failed: %v", err)
	}
	if len(res) != 2 {
		t.Fatalf("expected 2 down results, got %d", len(res))
	}
	if len(received) != 2 || received[0] != "2" || received[1] != "1" {
		t.Fatalf("expected reverse order [2,1], got %v", received)
	}
}

// Verify that versioned migrator records status_code and optional body in migration_runs for MigrateUp.
func TestMigrateUp_RecordsStatusAndBody(t *testing.T) {
	for _, save := range []bool{false, true} {
		t.Run(fmt.Sprintf("saveBody=%v", save), func(t *testing.T) {
			// HTTP server returns 200 and a small JSON body
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(200)
				_, _ = w.Write([]byte(`{"ok":true,"msg":"hello"}`))
			}))
			defer srv.Close()

			dir := t.TempDir()
			mig := "up:\n  name: only\n  env: { }\n  request:\n    method: GET\n    url: " + srv.URL + "\n  response:\n    result_code: [\"200\"]\n"
			if err := os.WriteFile(filepath.Join(dir, "001_only.yaml"), []byte(mig), 0o600); err != nil {
				t.Fatalf("write mig: %v", err)
			}
			ctx := context.WithValue(context.Background(), SaveResponseBodyKey, save)
			base := env.Env{Global: map[string]string{}}
			if _, err := MigrateUp(ctx, dir, base, 0); err != nil {
				t.Fatalf("migrate up: %v", err)
			}

			// Inspect migration_runs
			dbPath := filepath.Join(dir, store.DbFileName)
			st, err := store.Open(dbPath)
			if err != nil {
				t.Fatalf("open store: %v", err)
			}
			defer func() { _ = st.Close() }()
			rows, err := st.DB.Query(`SELECT status_code, body FROM migration_runs ORDER BY id ASC`)
			if err != nil {
				t.Fatalf("query runs: %v", err)
			}
			defer func() { _ = rows.Close() }()
			n := 0
			for rows.Next() {
				n++
				var code int
				var body sql.NullString
				if err := rows.Scan(&code, &body); err != nil {
					t.Fatalf("scan: %v", err)
				}
				if code != 200 {
					t.Fatalf("expected status_code 200, got %d", code)
				}
				if save {
					if !body.Valid || body.String == "" {
						t.Fatalf("expected non-empty body when saveBody=true, got %+v", body)
					}
				} else {
					if body.Valid && body.String != "" {
						t.Fatalf("expected empty/NULL body when saveBody=false, got %q", body.String)
					}
				}
			}
			if err := rows.Err(); err != nil {
				t.Fatalf("rows err: %v", err)
			}
			if n != 1 {
				t.Fatalf("expected 1 run row, got %d", n)
			}
		})
	}
}

// Ensure stored_env from Up is persisted to the store (basic verification)
func TestMigrateUp_StoresEnv_Persisted(t *testing.T) {
	var createCalls int
	// Server: up returns id
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/create" {
			createCalls++
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(200)
			_, _ = w.Write([]byte(`{"id":"abc"}`))
			return
		}
		w.WriteHeader(404)
	}))
	defer srv.Close()

	dir := t.TempDir()
	m1 := "up:\n  name: create\n  env: { }\n  request:\n    method: POST\n    url: " + srv.URL + "/create\n  response:\n    result_code: [\"200\"]\n    env_from:\n      rid: id\n"
	if err := os.WriteFile(filepath.Join(dir, "001_create.yaml"), []byte(m1), 0o600); err != nil {
		t.Fatalf("write m1: %v", err)
	}
	ctx := context.Background()
	base := env.Env{Global: map[string]string{}}
	if _, err := MigrateUp(ctx, dir, base, 0); err != nil {
		t.Fatalf("migrate up: %v", err)
	}
	if createCalls == 0 {
		t.Fatalf("expected create to be called")
	}
	// verify stored_env contains rid
	st, err := store.Open(filepath.Join(dir, store.DbFileName))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer func() { _ = st.Close() }()
	rows, err := st.DB.Query(`SELECT COUNT(1) FROM stored_env WHERE version=1 AND name='rid' AND value='abc'`)
	if err != nil {
		t.Fatalf("query stored_env: %v", err)
	}
	var cnt int
	if rows.Next() {
		if err := rows.Scan(&cnt); err != nil {
			t.Fatalf("scan: %v", err)
		}
	}
	_ = rows.Close()
	if cnt != 1 {
		t.Fatalf("expected 1 stored_env row for version=1 rid=abc, got %d", cnt)
	}
}

// Verify that stored_env entries are used by Down templating and are deleted after Down
func TestMigrateDown_UsesStoredEnvAndCleans(t *testing.T) {
	var delPath string
	// Server: up returns id; down DELETE must hit /resource/123
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := r.URL.Path
		if p == "/create" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(200)
			_, _ = w.Write([]byte(`{"id":"123"}`))
			return
		}
		if p == "/resource/123" && r.Method == http.MethodDelete {
			delPath = p
			w.WriteHeader(200)
			_, _ = w.Write([]byte(`{"ok":true}`))
			return
		}
		w.WriteHeader(404)
	}))
	defer srv.Close()

	dir := t.TempDir()
	mig := "up:\n  name: create resource\n  env: { }\n  request:\n    method: POST\n    url: " + srv.URL + "/create\n  response:\n    result_code: [\"200\"]\n    env_from:\n      rid: id\n\n" +
		"down:\n  name: delete resource\n  env: { }\n  method: DELETE\n  url: \"" + srv.URL + "/resource/{{.rid}}\"\n"
	if err := os.WriteFile(filepath.Join(dir, "001_create_and_delete.yaml"), []byte(mig), 0o600); err != nil {
		t.Fatalf("write mig: %v", err)
	}
	ctx := context.Background()
	base := env.Env{Global: map[string]string{}}
	// Apply up
	if _, err := MigrateUp(ctx, dir, base, 0); err != nil {
		t.Fatalf("migrate up: %v", err)
	}
	// Now perform down to 0, expecting DELETE with rid from store and cleanup
	if _, err := MigrateDown(ctx, dir, base, 0); err != nil {
		t.Fatalf("migrate down: %v", err)
	}
	if delPath != "/resource/123" {
		t.Fatalf("expected DELETE to /resource/123, got %s", delPath)
	}
	// ensure stored_env cleaned for version 1
	st, err := store.Open(filepath.Join(dir, store.DbFileName))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer func() { _ = st.Close() }()
	rows, err := st.DB.Query(`SELECT COUNT(1) FROM stored_env WHERE version=1`)
	if err != nil {
		t.Fatalf("query stored_env: %v", err)
	}
	var cnt int
	if rows.Next() {
		if err := rows.Scan(&cnt); err != nil {
			t.Fatalf("scan: %v", err)
		}
	}
	_ = rows.Close()
	if cnt != 0 {
		t.Fatalf("expected 0 stored_env rows after down, got %d", cnt)
	}
}
