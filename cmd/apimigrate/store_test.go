package main

import (
	"database/sql"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/loykin/apimigrate"
	"github.com/spf13/viper"
)

func countRuns(t *testing.T, dbPath string) (int, int) {
	t.Helper()
	st, err := apimigrate.OpenStore(dbPath)
	if err != nil {
		t.Fatalf("OpenStore: %v", err)
	}
	defer func() { _ = st.Close() }()
	rows, err := st.DB.Query(`SELECT status_code, body FROM migration_runs ORDER BY id ASC`)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	defer func() { _ = rows.Close() }()
	n := 0
	bodies := 0
	for rows.Next() {
		var code int
		var body sql.NullString
		if err := rows.Scan(&code, &body); err != nil {
			t.Fatalf("scan: %v", err)
		}
		n++
		if body.Valid && body.String != "" {
			bodies++
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("rows err: %v", err)
	}
	return n, bodies
}

// Ensure we record status code and (optionally) body, when enabled.
func TestStore_RecordsRuns_WithAndWithoutBody(t *testing.T) {
	for _, save := range []bool{false, true} {
		t.Run(fmt.Sprintf("saveBody=%v", save), func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(200)
				_, _ = w.Write([]byte(`{"ok":true,"msg":"hello"}`))
			}))
			defer srv.Close()

			tdir := t.TempDir()
			m1 := fmt.Sprintf(`---
up:
  name: v1
  request:
    method: GET
    url: %s/one
  response:
    result_code: ["200"]
`, srv.URL)
			_ = writeFile(t, tdir, "001_one.yaml", m1)

			cfg := fmt.Sprintf(`---
store:
  save_response_body: %v
migrate_dir: %s
`, save, tdir)
			cfgPath := writeFile(t, tdir, "config.yaml", cfg)

			v := viper.GetViper()
			v.Set("config", cfgPath)
			v.Set("v", false)
			if err := upCmd.RunE(upCmd, nil); err != nil {
				t.Fatalf("up: %v", err)
			}

			dbPath := filepath.Join(tdir, apimigrate.StoreDBFileName)
			runs, bodies := countRuns(t, dbPath)
			if runs != 1 {
				t.Fatalf("expected 1 run, got %d", runs)
			}
			if save {
				if bodies != 1 {
					t.Fatalf("expected 1 body when save=true, got %d", bodies)
				}
			} else {
				if bodies != 0 {
					t.Fatalf("expected 0 bodies when save=false, got %d", bodies)
				}
			}
		})
	}
}

// Verify that values extracted via env_from are automatically persisted to the store and that
// MigrateDown merges the stored env into the down environment for templating.
func TestStoreEnv_PersistAndUseInDown(t *testing.T) {
	var delPath string
	var createCalls int32
	// Server: create returns an id in JSON; delete expects the templated id in URL
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := r.URL.Path
		if strings.HasPrefix(p, "/create") {
			atomic.AddInt32(&createCalls, 1)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(200)
			_, _ = w.Write([]byte(`{"id":"123"}`))
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

	tdir := t.TempDir()
	up := fmt.Sprintf(`---
up:
  name: create resource
  request:
    method: POST
    url: %s/create
  response:
    result_code: ["200"]
    env_from:
      rid: id

down:
  name: delete resource
  method: DELETE
  url: %s/resource/{{.rid}}
`, srv.URL, srv.URL)
	_ = writeFile(t, tdir, "001_create.yaml", up)

	cfg := fmt.Sprintf(`---
migrate_dir: %s
`, tdir)
	cfgPath := writeFile(t, tdir, "config.yaml", cfg)

	v := viper.GetViper()
	v.Set("config", cfgPath)
	v.Set("v", false)
	v.Set("to", 0)
	// Apply up
	if err := upCmd.RunE(upCmd, nil); err != nil {
		t.Fatalf("up error: %v", err)
	}
	// Ensure run recorded
	if atomic.LoadInt32(&createCalls) == 0 {
		t.Fatalf("expected create to be called")
	}
	// Verify stored_env has the value after up
	dbPath := filepath.Join(tdir, apimigrate.StoreDBFileName)
	st, err := apimigrate.OpenStore(dbPath)
	if err != nil {
		t.Fatalf("OpenStore: %v", err)
	}
	defer func() { _ = st.Close() }()
	rows, err := st.DB.Query(`SELECT COUNT(1) FROM stored_env WHERE version=1 AND name='rid' AND value='123'`)
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
		t.Fatalf("expected 1 stored_env row for version=1 rid=123, got %d", cnt)
	}

	// Now down to 0 -> should DELETE with stored id 123 and remove stored rows
	if err := downCmd.RunE(downCmd, nil); err != nil {
		t.Fatalf("down error: %v", err)
	}
	if delPath != "/resource/123" {
		t.Fatalf("expected DELETE to /resource/123, got %s", delPath)
	}
	rows2, err := st.DB.Query(`SELECT COUNT(1) FROM stored_env WHERE version=1`)
	if err != nil {
		t.Fatalf("query stored_env after down: %v", err)
	}
	cnt = -1
	if rows2.Next() {
		if err := rows2.Scan(&cnt); err != nil {
			t.Fatalf("scan: %v", err)
		}
	}
	_ = rows2.Close()
	if cnt != 0 {
		t.Fatalf("expected 0 stored_env rows after down, got %d", cnt)
	}
}

// Verify that stored_env entries are available for templating in subsequent up migrations.
func TestStoreEnv_AvailableInNextUp(t *testing.T) {
	var seenPath string
	var createCalls int32
	// Server: first up returns id; second up should use it in URL
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := r.URL.Path
		if strings.HasPrefix(p, "/create") {
			atomic.AddInt32(&createCalls, 1)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(200)
			_, _ = w.Write([]byte(`{"id":"abc"}`))
			return
		}
		if strings.HasPrefix(p, "/use/") {
			seenPath = p
			w.WriteHeader(200)
			_, _ = w.Write([]byte(`{"ok":true}`))
			return
		}
		w.WriteHeader(404)
	}))
	defer srv.Close()

	tdir := t.TempDir()
	m1 := fmt.Sprintf(`---
up:
  name: create
  request:
    method: POST
    url: %s/create
  response:
    result_code: ["200"]
    env_from:
      rid: id
`, srv.URL)
	m2 := fmt.Sprintf(`---
up:
  name: use
  request:
    method: GET
    url: %s/use/{{.rid}}
  response:
    result_code: ["200"]
`, srv.URL)
	_ = writeFile(t, tdir, "001_create.yaml", m1)
	_ = writeFile(t, tdir, "002_use.yaml", m2)

	cfg := fmt.Sprintf(`---
migrate_dir: %s
`, tdir)
	cfgPath := writeFile(t, tdir, "config.yaml", cfg)

	v := viper.GetViper()
	v.Set("config", cfgPath)
	v.Set("v", false)
	v.Set("to", 0)
	if err := upCmd.RunE(upCmd, nil); err != nil {
		t.Fatalf("up error: %v", err)
	}
	if atomic.LoadInt32(&createCalls) == 0 {
		t.Fatalf("expected create to be called")
	}
	if seenPath != "/use/abc" {
		t.Fatalf("expected second up to use stored rid in URL, got %s", seenPath)
	}
}

func TestBuildStoreOptions_EmptyType_ReturnsNil(t *testing.T) {
	doc := ConfigDoc{Store: StoreConfig{Type: ""}}
	if got := doc.Store.ToStorOptions(); got != nil {
		t.Fatalf("expected nil for empty type, got %#v", got)
	}
}

func TestBuildStoreOptions_Postgres_WithDSN(t *testing.T) {
	doc := ConfigDoc{Store: StoreConfig{Type: apimigrate.DriverPostgres, Postgres: PostgresStoreConfig{DSN: "postgres://u:p@h:5432/db?sslmode=disable"}}}
	got := doc.Store.ToStorOptions()
	if got == nil {
		t.Fatalf("expected non-nil options")
	}
	if got.Backend != apimigrate.DriverPostgres {
		t.Fatalf("backend=%s, want postgres", got.Backend)
	}
	if got.PostgresDSN != "postgres://u:p@h:5432/db?sslmode=disable" {
		t.Fatalf("dsn=%q, want provided dsn", got.PostgresDSN)
	}
}

func TestBuildStoreOptions_Postgres_BuildFromComponents_Defaults(t *testing.T) {
	doc := ConfigDoc{Store: StoreConfig{Type: apimigrate.DriverPostgres, Postgres: PostgresStoreConfig{
		Host: "localhost", User: "user", Password: "pass", DBName: "db", // Port=0 -> default 5432, SSLMode empty -> disable
	}}}
	got := doc.Store.ToStorOptions()
	if got == nil || got.Backend != apimigrate.DriverPostgres {
		t.Fatalf("expected postgres backend, got %#v", got)
	}
	exp := "postgres://user:pass@localhost:5432/db?sslmode=disable"
	if got.PostgresDSN != exp {
		t.Fatalf("built dsn=%q, want %q", got.PostgresDSN, exp)
	}
}

func TestBuildStoreOptions_Postgres_Aliases(t *testing.T) {
	aliases := []string{"pg", "postgresql"}
	for _, a := range aliases {
		doc := ConfigDoc{Store: StoreConfig{Type: a, Postgres: PostgresStoreConfig{DSN: "postgres://u:p@h:5432/db?sslmode=disable"}}}
		got := doc.Store.ToStorOptions()
		if got == nil || got.Backend != apimigrate.DriverPostgres {
			t.Fatalf("alias %s: expected postgres backend, got %#v", a, got)
		}
	}
}

func TestBuildStoreOptions_SQLite_Path(t *testing.T) {
	doc := ConfigDoc{Store: StoreConfig{Type: "sqlite", SQLite: SQLiteStoreConfig{Path: "/tmp/x.db"}}}
	got := doc.Store.ToStorOptions()
	if got == nil || got.Backend != "sqlite" {
		t.Fatalf("expected sqlite backend, got %#v", got)
	}
	if got.SQLitePath != "/tmp/x.db" {
		t.Fatalf("sqlite path=%q, want /tmp/x.db", got.SQLitePath)
	}
}

func TestBuildStoreOptions_UnknownType_FallsBackToSQLite(t *testing.T) {
	doc := ConfigDoc{Store: StoreConfig{Type: "maria", SQLite: SQLiteStoreConfig{Path: "./foo.db"}}}
	got := doc.Store.ToStorOptions()
	if got == nil || got.Backend != "sqlite" || got.SQLitePath != "./foo.db" {
		t.Fatalf("fallback to sqlite mismatch, got %#v", got)
	}
}

// Sanity: ensure function is pure relative to input (no mutation of doc)
func TestBuildStoreOptions_DoesNotMutateInput(t *testing.T) {
	orig := ConfigDoc{Store: StoreConfig{Type: "postgres", Postgres: PostgresStoreConfig{Host: "h", User: "u", Password: "p", DBName: "d"}}}
	cp := orig
	_ = orig.Store.ToStorOptions()
	if !reflect.DeepEqual(orig, cp) {
		t.Fatalf("expected input doc to remain unchanged")
	}
}

func TestOpenStoreFromOptions_DefaultNil_UsesDirSqlite(t *testing.T) {
	dir := t.TempDir()
	st, err := apimigrate.OpenStoreFromOptions(dir, nil)
	if err != nil {
		t.Fatalf("OpenStoreFromOptions(nil) err: %v", err)
	}
	defer func() { _ = st.Close() }()
	v, err := st.CurrentVersion()
	if err != nil {
		t.Fatalf("CurrentVersion: %v", err)
	}
	if v != 0 {
		t.Fatalf("expected version 0 for new sqlite store, got %d", v)
	}
}

func TestOpenStoreFromOptions_SQLitePath_UsesPath(t *testing.T) {
	dir := t.TempDir()
	custom := filepath.Join(dir, "custom.db")
	opts := &StoreOptionsShimSQLite{Type: "sqlite", Path: custom}
	// Use config doc to generate options, then open
	doc := ConfigDoc{Store: StoreConfig{Type: opts.Type, SQLite: SQLiteStoreConfig{Path: opts.Path}}}
	so := doc.Store.ToStorOptions()
	st, err := apimigrate.OpenStoreFromOptions(dir, so)
	if err != nil {
		t.Fatalf("OpenStoreFromOptions(sqlite path) err: %v", err)
	}
	defer func() { _ = st.Close() }()
	// Ensure schema can be queried
	if _, err := st.CurrentVersion(); err != nil {
		t.Fatalf("CurrentVersion on custom path: %v", err)
	}
}

func TestOpenStoreFromOptions_PostgresMissingDSN_ReturnsError(t *testing.T) {
	dir := t.TempDir()
	doc := ConfigDoc{Store: StoreConfig{Type: apimigrate.DriverPostgres, Postgres: PostgresStoreConfig{DSN: ""}}}
	so := doc.Store.ToStorOptions()
	// buildStoreOptionsFromDoc returns postgres backend with empty DSN if host also empty
	// OpenStoreFromOptions should error
	_, err := apimigrate.OpenStoreFromOptions(dir, so)
	if err == nil {
		t.Fatalf("expected error when DSN is empty for postgres backend")
	}
}

type StoreOptionsShimSQLite struct {
	Type string
	Path string
}

func TestBuildStoreOptions_TableNames_PassThrough(t *testing.T) {
	doc := ConfigDoc{Store: StoreConfig{
		Type:                    "sqlite",
		SQLite:                  SQLiteStoreConfig{Path: "/tmp/x.db"},
		TableSchemaMigrations:   "sm_custom",
		TableMigrationRuns:      "mr_custom",
		TableStoredEnv:          "se_custom",
		IndexStoredEnvByVersion: "idx_se_ver",
	}}
	got := doc.Store.ToStorOptions()
	if got == nil {
		t.Fatalf("expected non-nil store options")
	}
	if got.TableSchemaMigrations != "sm_custom" || got.TableMigrationRuns != "mr_custom" || got.TableStoredEnv != "se_custom" || got.IndexStoredEnvByVersion != "idx_se_ver" {
		t.Fatalf("table names not passed through: %#v", got)
	}
}

func TestOpenStoreFromOptions_SQLite_CustomTableNames(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "named.db")
	doc := ConfigDoc{Store: StoreConfig{
		Type:                    "sqlite",
		SQLite:                  SQLiteStoreConfig{Path: dbPath},
		TableSchemaMigrations:   "sm_custom",
		TableMigrationRuns:      "mr_custom",
		TableStoredEnv:          "se_custom",
		IndexStoredEnvByVersion: "idx_se_ver",
	}}
	opts := doc.Store.ToStorOptions()
	st, err := apimigrate.OpenStoreFromOptions(dir, opts)
	if err != nil {
		t.Fatalf("OpenStoreFromOptions with names: %v", err)
	}
	defer func() { _ = st.Close() }()
	// Check sqlite_master for custom table names
	must := []string{"sm_custom", "mr_custom", "se_custom"}
	for _, tbl := range must {
		row := st.DB.QueryRow(`SELECT name FROM sqlite_master WHERE type='table' AND name=?`, tbl)
		var name string
		if err := row.Scan(&name); err != nil {
			t.Fatalf("expected table %s to exist: %v", tbl, err)
		}
		if name != tbl {
			t.Fatalf("expected table %s, got %s", tbl, name)
		}
	}
}

func TestBuildStoreOptions_TablePrefix_ComputesNames(t *testing.T) {
	doc := ConfigDoc{Store: StoreConfig{
		Type:        "sqlite",
		SQLite:      SQLiteStoreConfig{Path: "/tmp/x.db"},
		TablePrefix: "app1",
	}}
	got := doc.Store.ToStorOptions()
	if got == nil {
		t.Fatalf("expected non-nil store options")
	}
	if got.TableSchemaMigrations != "app1_schema_migrations" || got.TableMigrationRuns != "app1_migration_log" || got.TableStoredEnv != "app1_stored_env" {
		t.Fatalf("prefix-derived names mismatch: %#v", got)
	}
}

func TestOpenStoreFromOptions_SQLite_TablePrefix_CreatesTables(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "prefix.db")
	doc := ConfigDoc{Store: StoreConfig{
		Type:        "sqlite",
		SQLite:      SQLiteStoreConfig{Path: dbPath},
		TablePrefix: "pfx",
	}}
	opts := doc.Store.ToStorOptions()
	st, err := apimigrate.OpenStoreFromOptions(dir, opts)
	if err != nil {
		t.Fatalf("OpenStoreFromOptions: %v", err)
	}
	defer func() { _ = st.Close() }()
	must := []string{"pfx_schema_migrations", "pfx_migration_log", "pfx_stored_env"}
	for _, tbl := range must {
		row := st.DB.QueryRow(`SELECT name FROM sqlite_master WHERE type='table' AND name=?`, tbl)
		var name string
		if err := row.Scan(&name); err != nil {
			t.Fatalf("expected table %s to exist: %v", tbl, err)
		}
		if name != tbl {
			t.Fatalf("expected %s, got %s", tbl, name)
		}
	}
}
