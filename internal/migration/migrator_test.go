package migration

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/loykin/apimigrate/internal/env"
	"github.com/loykin/apimigrate/internal/store"
)

func TestDecodeTaskYAML_Valid(t *testing.T) {
	// Minimal but representative YAML including Up and Down
	yaml := strings.NewReader(
		"up:\n  name: test-up\n  env: { X: 'y' }\n  request:\n    method: GET\n    url: http://example.com\n  response:\n    result_code: ['200']\n" +
			"down:\n  name: test-down\n  env: {}\n  method: DELETE\n  url: http://example.com\n",
	)

	tk, err := decodeTaskYAML(yaml)
	if err != nil {
		t.Fatalf("unexpected error decoding: %v", err)
	}
	// Basic assertions
	if tk.Up.Name != "test-up" {
		t.Fatalf("expected up.name 'test-up', got %q", tk.Up.Name)
	}
	if tk.Up.Request.Method != http.MethodGet || tk.Up.Request.URL != "http://example.com" {
		t.Fatalf("unexpected up.request: method=%q url=%q", tk.Up.Request.Method, tk.Up.Request.URL)
	}
	if len(tk.Up.Response.ResultCode) != 1 || tk.Up.Response.ResultCode[0] != "200" {
		t.Fatalf("unexpected up.response result_code: %v", tk.Up.Response.ResultCode)
	}
	if tk.Down.Name != "test-down" || tk.Down.Method != http.MethodDelete || tk.Down.URL != "http://example.com" {
		t.Fatalf("unexpected down fields: %+v", tk.Down)
	}
}

func TestDecodeTaskYAML_Invalid(t *testing.T) {
	bad := strings.NewReader(":: not yaml ::")
	_, err := decodeTaskYAML(bad)
	if err == nil {
		t.Fatalf("expected error for invalid yaml, got nil")
	}
}

func TestLoadTaskFromFile_Success(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "001_sample.yaml")
	content := "up:\n  request: { method: POST, url: http://localhost }\n  response: { result_code: ['200'] }\n"
	if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}

	tk, err := loadTaskFromFile(p)
	if err != nil {
		t.Fatalf("unexpected error loading from file: %v", err)
	}
	if strings.ToUpper(tk.Up.Request.Method) != http.MethodPost {
		t.Fatalf("expected method POST, got %q", tk.Up.Request.Method)
	}
}

func TestLoadTaskFromFile_NotFound(t *testing.T) {
	_, err := loadTaskFromFile(filepath.Join(t.TempDir(), "missing.yaml"))
	if err == nil {
		t.Fatalf("expected error for missing file, got nil")
	}
}

// Ensure migration_runs.failed reflects error/success for Up and Down.
func TestMigrator_RecordsFailedFlag_OnEnvMissingFail(t *testing.T) {
	// Server returns 200 with only {"id":"x"} for up; down endpoint just 200
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		if r.URL.Path == "/create" {
			_, _ = w.Write([]byte(`{"id":"x"}`))
			return
		}
	}))
	defer srv.Close()

	dir := t.TempDir()
	// Up requires both rid and missing -> with env_missing=fail this should fail
	m1 := "up:\n  name: create\n  env: { }\n  request:\n    method: POST\n    url: " + srv.URL + "/create\n  response:\n    result_code: [\"200\"]\n    env_missing: fail\n    env_from:\n      rid: id\n      nope: missing.path\n"
	if err := os.WriteFile(filepath.Join(dir, "001_up_fail.yaml"), []byte(m1), 0o600); err != nil {
		t.Fatalf("write m1: %v", err)
	}

	ctx := context.Background()
	base := env.Env{Global: map[string]string{}}
	st := openTestStore(t, filepath.Join(dir, store.DbFileName))
	defer func() { _ = st.Close() }()

	// Run MigrateUp; expect error and migration_runs.failed=true
	_, err := (&Migrator{Dir: dir, Env: base, Store: *st}).MigrateUp(ctx, 0)
	if err == nil {
		t.Fatalf("expected migrate up to fail due to env_missing=fail")
	}
	// Inspect migration_runs
	row := st.DB.QueryRow(`SELECT status_code, failed FROM migration_runs ORDER BY id DESC LIMIT 1`)
	var code int
	var failed bool
	if err := row.Scan(&code, &failed); err != nil {
		t.Fatalf("scan run: %v", err)
	}
	if code != 200 || !failed {
		t.Fatalf("expected status=200 and failed=true, got code=%d failed=%v", code, failed)
	}

	// Now, in a separate directory, create a migration that succeeds and validate down records failed=false
	dir2 := t.TempDir()
	m2 := "up:\n  name: ok\n  env: { }\n  request:\n    method: GET\n    url: " + srv.URL + "\n  response:\n    result_code: [\"200\"]\n\n" +
		"down:\n  name: cleanup\n  env: { }\n  method: DELETE\n  url: " + srv.URL + "\n"
	if err := os.WriteFile(filepath.Join(dir2, "001_ok.yaml"), []byte(m2), 0o600); err != nil {
		t.Fatalf("write m2: %v", err)
	}
	st2 := openTestStore(t, filepath.Join(dir2, store.DbFileName))
	defer func() { _ = st2.Close() }()
	if _, err := (&Migrator{Dir: dir2, Env: base, Store: *st2}).MigrateUp(ctx, 0); err != nil {
		t.Fatalf("migrate up v2: %v", err)
	}
	if _, err := (&Migrator{Dir: dir2, Env: base, Store: *st2}).MigrateDown(ctx, 0); err != nil {
		t.Fatalf("migrate down: %v", err)
	}
	// Re-run a focused query for last down row in dir2 store
	row2 := st2.DB.QueryRow(`SELECT direction, failed FROM migration_runs ORDER BY id DESC LIMIT 1`)
	var direction string
	var f2 bool
	if err := row2.Scan(&direction, &f2); err != nil {
		t.Fatalf("scan last: %v", err)
	}
	if direction != "down" || f2 {
		t.Fatalf("expected last run to be down with failed=false, got dir=%s failed=%v", direction, f2)
	}
}
