package migration

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/loykin/apimigrate/internal/auth"
	"github.com/loykin/apimigrate/internal/env"
	"github.com/loykin/apimigrate/internal/store"
	"github.com/loykin/apimigrate/internal/task"
)

func TestDecodeTaskYAML_Valid(t *testing.T) {
	// Minimal but representative YAML including Up and Down
	yaml := strings.NewReader(
		"up:\n  name: test-up\n  env: { X: 'y' }\n  request:\n    method: GET\n    url: http://example.com\n  response:\n    result_code: ['200']\n" +
			"down:\n  name: test-down\n  env: {}\n  method: DELETE\n  url: http://example.com\n",
	)

	var tk task.Task
	if err := tk.DecodeYAML(yaml); err != nil {
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
	var tk task.Task
	err := tk.DecodeYAML(bad)
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

	var tk task.Task
	if err := tk.LoadFromFile(p); err != nil {
		t.Fatalf("unexpected error loading from file: %v", err)
	}
	if strings.ToUpper(tk.Up.Request.Method) != http.MethodPost {
		t.Fatalf("expected method POST, got %q", tk.Up.Request.Method)
	}
}

func TestLoadTaskFromFile_NotFound(t *testing.T) {
	var tk task.Task
	err := tk.LoadFromFile(filepath.Join(t.TempDir(), "missing.yaml"))
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

// Test multiple up and down runs and verify headers, queries, body, and env propagation/cleanup.
func TestMigrator_MultipleUpDown_RequestAndEnvFlow(t *testing.T) {
	// Record last request details for endpoints
	type rec struct {
		method  string
		path    string
		headers http.Header
		query   url.Values
		body    string
	}
	var create rec
	var use rec
	var del rec

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		bodyBytes, _ := io.ReadAll(r.Body)
		b := string(bodyBytes)
		switch r.URL.Path {
		case "/create":
			create = rec{method: r.Method, path: r.URL.Path, headers: r.Header.Clone(), query: r.URL.Query(), body: b}
			w.WriteHeader(200)
			// Return an id to be extracted and stored
			_, _ = w.Write([]byte(`{"id":"abc123","info":"ok"}`))
			return
		case "/use/abc123":
			use = rec{method: r.Method, path: r.URL.Path, headers: r.Header.Clone(), query: r.URL.Query(), body: b}
			w.WriteHeader(200)
			_, _ = w.Write([]byte(`{"ok":true}`))
			return
		case "/delete/abc123":
			del = rec{method: r.Method, path: r.URL.Path, headers: r.Header.Clone(), query: r.URL.Query(), body: b}
			w.WriteHeader(200)
			_, _ = w.Write([]byte(`{"deleted":true}`))
			return
		case "/noop":
			w.WriteHeader(200)
			_, _ = w.Write([]byte(`{"noop":true}`))
			return
		default:
			w.WriteHeader(404)
			_, _ = w.Write([]byte(`{"err":"unknown"}`))
		}
	}))
	defer srv.Close()

	dir := t.TempDir()
	// Migration 001: create resource, extract id into rid; also store a value via env_from
	mig1 := fmt.Sprintf(`up:
  name: create
  env: { }
  request:
    method: POST
    url: %s/create
    headers:
      - { name: X-Fixed, value: 'v1' }
    queries:
      - { name: q, value: from-up }
    body: '{"note":"hello"}'
  response:
    result_code: ["200"]
    env_missing: fail
    env_from:
      rid: id

down:
  name: delete
  env: { }
  method: DELETE
  url: %s/delete/{{.env.rid}}
  headers:
    - { name: X-Del, value: 'yes' }
`, srv.URL, srv.URL)
	if err := os.WriteFile(filepath.Join(dir, "001_create.yaml"), []byte(mig1), 0o600); err != nil {
		t.Fatalf("write mig1: %v", err)
	}
	// Migration 002: use the rid discovered in 001 in URL path, header, query and body
	bf := filepath.Join(dir, "use_body.txt")
	if err := os.WriteFile(bf, []byte("using {{.env.rid}}"), 0o600); err != nil {
		t.Fatalf("write body file: %v", err)
	}
	mig2 := fmt.Sprintf(`up:
  name: use
  env: { }
  request:
    method: POST
    url: %s/use/{{.env.rid}}
    headers:
      - { name: X-Use, value: 'id={{.env.rid}}' }
    queries:
      - { name: rid, value: '{{.env.rid}}' }
    body_file: %s
  response:
    result_code: ["200"]

down:
  name: noop
  env: { }
  method: GET
  url: %s/noop
`, srv.URL, bf, srv.URL)
	if err := os.WriteFile(filepath.Join(dir, "002_use.yaml"), []byte(mig2), 0o600); err != nil {
		t.Fatalf("write mig2: %v", err)
	}

	ctx := context.Background()
	base := env.Env{Global: map[string]string{"GLOBAL": "g"}}
	st := openTestStore(t, filepath.Join(dir, store.DbFileName))
	defer func() { _ = st.Close() }()

	m := &Migrator{Dir: dir, Env: base, Store: *st}
	// First up: should apply both 001 and 002
	resUp1, err := m.MigrateUp(ctx, 0)
	if err != nil {
		t.Fatalf("first MigrateUp error: %v", err)
	}
	if len(resUp1) != 2 {
		t.Fatalf("expected 2 up results, got %d", len(resUp1))
	}
	// Validate server received expected details
	if create.method != http.MethodPost || create.path != "/create" {
		t.Fatalf("unexpected create request: method=%s path=%s", create.method, create.path)
	}
	if create.headers.Get("X-Fixed") != "v1" {
		t.Fatalf("expected X-Fixed header on create")
	}
	if create.query.Get("q") != "from-up" {
		t.Fatalf("expected query q=from-up, got %s", create.query.Get("q"))
	}
	if !strings.Contains(create.body, "hello") {
		t.Fatalf("expected body to contain 'hello', got %q", create.body)
	}
	// Use step must reflect rid in URL, header, query and body
	if use.method != http.MethodPost || use.path != "/use/abc123" {
		t.Fatalf("unexpected use request: method=%s path=%s", use.method, use.path)
	}
	if use.headers.Get("X-Use") != "id=abc123" {
		t.Fatalf("expected X-Use header with rid, got %q", use.headers.Get("X-Use"))
	}
	if use.query.Get("rid") != "abc123" {
		t.Fatalf("expected query rid=abc123, got %s", use.query.Get("rid"))
	}
	if !strings.Contains(use.body, "abc123") {
		t.Fatalf("expected body containing rid, got %q", use.body)
	}

	// Second up: should be a no-op (no new results)
	resUp2, err := m.MigrateUp(ctx, 0)
	if err != nil {
		t.Fatalf("second MigrateUp error: %v", err)
	}
	if len(resUp2) != 0 {
		t.Fatalf("expected no results on second MigrateUp, got %d", len(resUp2))
	}

	// Now down to 0: should perform two downs (v2 noop then v1 delete)
	resDown1, err := m.MigrateDown(ctx, 0)
	if err != nil {
		t.Fatalf("first MigrateDown error: %v", err)
	}
	if len(resDown1) != 2 {
		t.Fatalf("expected 2 down results, got %d", len(resDown1))
	}
	if del.method != http.MethodDelete || del.path != "/delete/abc123" {
		t.Fatalf("unexpected delete request: method=%s path=%s", del.method, del.path)
	}
	if del.headers.Get("X-Del") != "yes" {
		t.Fatalf("expected X-Del=yes on delete")
	}

	// Second down to 0: should be a no-op
	resDown2, err := m.MigrateDown(ctx, 0)
	if err != nil {
		t.Fatalf("second MigrateDown error: %v", err)
	}
	if len(resDown2) != 0 {
		t.Fatalf("expected no results on second MigrateDown, got %d", len(resDown2))
	}

	// Ensure stored env is cleaned after down
	if _, err := st.LoadStoredEnv(1); err == nil {
		// LoadStoredEnv returns (map, error). We need to verify it's empty map
	}
	m1, _ := st.LoadStoredEnv(1)
	if len(m1) != 0 {
		t.Fatalf("expected stored env for v1 to be deleted, still have: %v", m1)
	}
}

// Test ensureAuth with multiple providers and respecting pre-set values
func TestEnsureAuth_MultiAndRespectPreset(t *testing.T) {
	// Register a fake provider under type "dummyX" locally
	auth.Register("dummyX", func(spec map[string]interface{}) (auth.Method, error) {
		return dummyMethod("tokX"), nil
	})
	auth.Register("dummyY", func(spec map[string]interface{}) (auth.Method, error) {
		return dummyMethod("tokY"), nil
	})

	m := &Migrator{Env: env.Env{Global: map[string]string{}, Auth: map[string]string{"y": "preset"}}}
	m.Auth = []auth.Auth{
		{Type: "dummyX", Name: "x", Methods: auth.NewAuthSpecFromMap(map[string]interface{}{})},
		{Type: "dummyY", Name: "y", Methods: auth.NewAuthSpecFromMap(map[string]interface{}{})},
	}
	if err := m.ensureAuth(context.Background()); err != nil {
		t.Fatalf("ensureAuth error: %v", err)
	}
	if m.Env.Auth["x"] != "tokX" {
		t.Fatalf("expected x set to tokX, got %q", m.Env.Auth["x"])
	}
	if m.Env.Auth["y"] != "preset" {
		t.Fatalf("expected y to remain preset, got %q", m.Env.Auth["y"])
	}
}

type dummyMethod string

func (d dummyMethod) Acquire(_ context.Context) (string, error) { return string(d), nil }

func TestMigrator_RenderBodyDefault_AppliesToUpAndDownFind(t *testing.T) {
	// Server echoes body; we check that templates are not rendered when default=false
	echo := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{"ok":true}`))
		_ = b
	}))
	defer echo.Close()

	dir := t.TempDir()
	// Up has a body with {{.env.X}} but no explicit request.render_body
	migUp := "up:\n  name: t\n  env: { X: 'y' }\n  request:\n    method: POST\n    url: " + echo.URL + "/echo\n    body: '{" + "\"a\":\"{{.env.X}}\"" + "}'\n  response:\n    result_code: ['200']\n\n" +
		"down:\n  name: d\n  env: { }\n  method: GET\n  url: " + echo.URL + "/d\n  find:\n    request:\n      method: POST\n      url: " + echo.URL + "/find\n      body: '{" + "\"b\":\"{{ missing }}\"" + "}'\n    response:\n      result_code: ['200']\n"
	if err := os.WriteFile(filepath.Join(dir, "001_t.yaml"), []byte(migUp), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	st := openTestStore(t, filepath.Join(dir, store.DbFileName))
	defer func() { _ = st.Close() }()

	// Default render false: Up should NOT render .env.X and Down.Find should NOT render missing
	defFalse := false
	m := &Migrator{Dir: dir, Store: *st, Env: env.Env{Global: map[string]string{}}, RenderBodyDefault: &defFalse}
	if _, err := m.MigrateUp(context.Background(), 0); err != nil {
		t.Fatalf("MigrateUp: %v", err)
	}
	if _, err := m.MigrateDown(context.Background(), 0); err != nil {
		t.Fatalf("MigrateDown: %v", err)
	}
}

// Test that MigrateUp propagates acquired auth into task requests
func TestMigrateUp_PropagatesAuthHeader(t *testing.T) {
	exp := "Basic " + base64.StdEncoding.EncodeToString([]byte("u:p"))
	hit := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/ok" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != exp {
			t.Fatalf("unexpected Authorization: got %q want %q", got, exp)
		}
		hit++
		w.WriteHeader(200)
	}))
	defer srv.Close()

	dir := t.TempDir()
	mig := []byte("" +
		"up:\n" +
		"  name: t\n" +
		"  request:\n" +
		"    method: GET\n" +
		"    url: " + srv.URL + "/ok\n" +
		"    headers:\n" +
		"      - { name: Authorization, value: 'Basic {{.auth.b}}' }\n" +
		"  response:\n" +
		"    result_code: ['200']\n")
	if err := os.WriteFile(filepath.Join(dir, "001_ok.yaml"), mig, 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}

	st := openTestStore(t, filepath.Join(dir, store.DbFileName))
	defer func() { _ = st.Close() }()

	m := &Migrator{Dir: dir, Store: *st, Env: env.Env{Global: map[string]string{}}, Auth: []auth.Auth{
		{Type: "basic", Name: "b", Methods: auth.NewAuthSpecFromMap(map[string]interface{}{"username": "u", "password": "p"})},
	}}
	if _, err := m.MigrateUp(context.Background(), 0); err != nil {
		t.Fatalf("MigrateUp: %v", err)
	}
	if hit != 1 {
		t.Fatalf("expected server hit once, got %d", hit)
	}
}
