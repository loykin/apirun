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

	"github.com/loykin/apimigrate/internal/store"
	"github.com/loykin/apimigrate/pkg/env"
)

func openTestStore(t *testing.T, dbPath string) *store.Store {
	t.Helper()
	cfg := store.Config{Driver: store.DriverSqlite, DriverConfig: &store.SqliteConfig{Path: dbPath}}
	st := &store.Store{}
	if err := st.Connect(cfg); err != nil {
		t.Fatalf("connect store: %v", err)
	}
	return st
}

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
	base := env.Env{Global: env.FromStringMap(map[string]string{})}
	// Apply both
	st := openTestStore(t, filepath.Join(dir, store.DbFileName))
	defer func() { _ = st.Close() }()
	if _, err := (&Migrator{Dir: dir, Env: &base, Store: *st}).MigrateUp(ctx, 0); err != nil {
		t.Fatalf("migrate up failed: %v", err)
	}
	// Rollback to 0 (all downs), expect order 2 then 1
	res, err := (&Migrator{Dir: dir, Env: &base, Store: *st}).MigrateDown(ctx, 0)
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
			ctx := context.Background()
			base := env.Env{Global: env.FromStringMap(map[string]string{})}
			st := openTestStore(t, filepath.Join(dir, store.DbFileName))
			defer func() { _ = st.Close() }()
			if _, err := (&Migrator{Dir: dir, Env: &base, Store: *st, SaveResponseBody: save}).MigrateUp(ctx, 0); err != nil {
				t.Fatalf("migrate up: %v", err)
			}

			// Inspect migration_runs
			dbPath := filepath.Join(dir, store.DbFileName)
			st2 := openTestStore(t, dbPath)
			defer func() { _ = st2.Close() }()
			rows, err := st2.DB.Query(`SELECT status_code, body FROM migration_runs ORDER BY id ASC`)
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
	base := env.Env{Global: env.FromStringMap(map[string]string{})}
	st := openTestStore(t, filepath.Join(dir, store.DbFileName))
	defer func() { _ = st.Close() }()
	if _, err := (&Migrator{Dir: dir, Env: &base, Store: *st}).MigrateUp(ctx, 0); err != nil {
		t.Fatalf("migrate up: %v", err)
	}
	if createCalls == 0 {
		t.Fatalf("expected create to be called")
	}
	// verify stored_env contains rid
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
		"down:\n  name: delete resource\n  env: { }\n  method: DELETE\n  url: \"" + srv.URL + "/resource/{{.env.rid}}\"\n"
	if err := os.WriteFile(filepath.Join(dir, "001_create_and_delete.yaml"), []byte(mig), 0o600); err != nil {
		t.Fatalf("write mig: %v", err)
	}
	ctx := context.Background()
	base := env.Env{Global: env.FromStringMap(map[string]string{})}
	// Apply up
	st := openTestStore(t, filepath.Join(dir, store.DbFileName))
	defer func() { _ = st.Close() }()
	if _, err := (&Migrator{Dir: dir, Env: &base, Store: *st}).MigrateUp(ctx, 0); err != nil {
		t.Fatalf("migrate up: %v", err)
	}
	// Now perform down to 0, expecting DELETE with rid from store and cleanup
	if _, err := (&Migrator{Dir: dir, Env: &base, Store: *st}).MigrateDown(ctx, 0); err != nil {
		t.Fatalf("migrate down: %v", err)
	}
	if delPath != "/resource/123" {
		t.Fatalf("expected DELETE to /resource/123, got %s", delPath)
	}
	// ensure stored_env cleaned for version 1
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

// Additional tests to cover planning and store options behavior introduced/refactored recently.
func TestMigrateUp_TargetVersionPlanning(t *testing.T) {
	calls := make(map[string]int)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls[r.URL.Path]++
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	dir := t.TempDir()
	m1 := "up:\n  name: v1\n  env: { }\n  request:\n    method: GET\n    url: " + srv.URL + "/v1\n  response:\n    result_code: [\"200\"]\n"
	m2 := "up:\n  name: v2\n  env: { }\n  request:\n    method: GET\n    url: " + srv.URL + "/v2\n  response:\n    result_code: [\"200\"]\n"
	m3 := "up:\n  name: v3\n  env: { }\n  request:\n    method: GET\n    url: " + srv.URL + "/v3\n  response:\n    result_code: [\"200\"]\n"
	if err := os.WriteFile(filepath.Join(dir, "001_v1.yaml"), []byte(m1), 0o600); err != nil {
		t.Fatalf("write m1: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "002_v2.yaml"), []byte(m2), 0o600); err != nil {
		t.Fatalf("write m2: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "003_v3.yaml"), []byte(m3), 0o600); err != nil {
		t.Fatalf("write m3: %v", err)
	}

	ctx := context.Background()
	base := env.Env{Global: env.FromStringMap(map[string]string{})}

	// Apply up to version 2 only
	st := openTestStore(t, filepath.Join(dir, store.DbFileName))
	defer func() { _ = st.Close() }()
	if _, err := (&Migrator{Dir: dir, Env: &base, Store: *st}).MigrateUp(ctx, 2); err != nil {
		t.Fatalf("migrate up to 2: %v", err)
	}
	// Verify only v1 and v2 were called
	if calls["/v1"] != 1 || calls["/v2"] != 1 || calls["/v3"] != 0 {
		t.Fatalf("expected calls v1=1 v2=1 v3=0, got: %v", calls)
	}
	// Verify store current version is 2
	cur, err := st.CurrentVersion()
	if err != nil {
		t.Fatalf("CurrentVersion: %v", err)
	}
	if cur != 2 {
		t.Fatalf("expected current version 2, got %d", cur)
	}

	// Now apply remaining (to=0 means all)
	if _, err := (&Migrator{Dir: dir, Env: &base, Store: *st}).MigrateUp(ctx, 0); err != nil {
		t.Fatalf("migrate up all: %v", err)
	}
	if calls["/v3"] != 1 {
		t.Fatalf("expected v3 to be called once after final up, got: %v", calls)
	}
	cur, err = st.CurrentVersion()
	if err != nil {
		t.Fatalf("CurrentVersion: %v", err)
	}
	if cur != 3 {
		t.Fatalf("expected current version 3, got %d", cur)
	}
}

func TestMigrate_StoreOptions_ExplicitSQLitePath(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	dir := t.TempDir()
	m := "up:\n  name: only\n  env: { }\n  request:\n    method: GET\n    url: " + srv.URL + "\n  response:\n    result_code: [\"200\"]\n"
	if err := os.WriteFile(filepath.Join(dir, "001_only.yaml"), []byte(m), 0o600); err != nil {
		t.Fatalf("write mig: %v", err)
	}

	customDir := t.TempDir()
	customPath := filepath.Join(customDir, "custom.db")
	base := env.Env{Global: env.FromStringMap(map[string]string{})}
	// open custom store directly and inject into migrator
	st := openTestStore(t, customPath)
	defer func() { _ = st.Close() }()
	if _, err := (&Migrator{Dir: dir, Env: &base, Store: *st}).MigrateUp(context.Background(), 0); err != nil {
		t.Fatalf("migrate up: %v", err)
	}
	if calls != 1 {
		t.Fatalf("expected exactly one call, got %d", calls)
	}
	// Verify DB exists at custom path, and default path does not exist
	if _, err := os.Stat(customPath); err != nil {
		t.Fatalf("expected custom sqlite DB at %s, stat err: %v", customPath, err)
	}
	defaultPath := filepath.Join(dir, store.DbFileName)
	if _, err := os.Stat(defaultPath); err == nil {
		t.Fatalf("did not expect default DB at %s when custom path is set", defaultPath)
	}
	// Inspect migration_runs in custom DB
	st2 := openTestStore(t, customPath)
	defer func() { _ = st2.Close() }()
	rows, err := st2.DB.Query(`SELECT COUNT(1) FROM migration_runs`)
	if err != nil {
		t.Fatalf("query runs: %v", err)
	}
	var cnt int
	if rows.Next() {
		if err := rows.Scan(&cnt); err != nil {
			t.Fatalf("scan: %v", err)
		}
	}
	_ = rows.Close()
	if cnt != 1 {
		t.Fatalf("expected 1 run row, got %d", cnt)
	}
}

func TestOpenStore_DefaultOnManualPath(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, store.DbFileName)
	st := openTestStore(t, path)
	_ = st.Close()
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected sqlite db created: %v", err)
	}
}

func TestOpenStore_SQLiteCustomPath(t *testing.T) {
	customDir := t.TempDir()
	customPath := filepath.Join(customDir, "custom.db")
	st := openTestStore(t, customPath)
	_ = st.Close()
	if _, err := os.Stat(customPath); err != nil {
		t.Fatalf("expected custom sqlite db at %s: %v", customPath, err)
	}
}
