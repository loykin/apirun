package store

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"
)

// helper to open a store in a temporary file path
func openTempStore(t *testing.T) *Store {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, DbFileName)
	st := &Store{}
	cfg := Config{Driver: DriverSqlite, DriverConfig: &SqliteConfig{Path: path}}
	if err := st.Connect(cfg); err != nil {
		t.Fatalf("failed to open store: %v", err)
	}
	t.Cleanup(func() { _ = st.Close(); _ = os.Remove(path) })
	return st
}

func TestOpenAndEmptyState(t *testing.T) {
	st := openTempStore(t)
	// EnsureSchema should be idempotent
	if err := st.EnsureSchema(); err != nil {
		t.Fatalf("EnsureSchema failed: %v", err)
	}
	// No versions yet
	v, err := st.CurrentVersion()
	if err != nil {
		t.Fatalf("CurrentVersion error: %v", err)
	}
	if v != 0 {
		t.Fatalf("expected CurrentVersion=0, got %d", v)
	}
	list, err := st.ListApplied()
	if err != nil {
		t.Fatalf("ListApplied error: %v", err)
	}
	if len(list) != 0 {
		t.Fatalf("expected empty applied list, got %v", list)
	}
}

func TestApplyListCurrentIsApplied(t *testing.T) {
	st := openTempStore(t)
	// Apply out of order
	for _, v := range []int{1, 3, 2} {
		if err := st.Apply(v); err != nil {
			t.Fatalf("Apply(%d) err: %v", v, err)
		}
	}
	// Idempotent apply should not error
	if err := st.Apply(2); err != nil {
		t.Fatalf("re-Apply err: %v", err)
	}
	// IsApplied checks
	ap, err := st.IsApplied(2)
	if err != nil || !ap {
		t.Fatalf("IsApplied(2) => %v, %v; want true, nil", ap, err)
	}
	ap, err = st.IsApplied(99)
	if err != nil || ap {
		t.Fatalf("IsApplied(99) => %v, %v; want false, nil", ap, err)
	}
	// CurrentVersion should be 3
	cur, err := st.CurrentVersion()
	if err != nil {
		t.Fatalf("CurrentVersion err: %v", err)
	}
	if cur != 3 {
		t.Fatalf("CurrentVersion=%d, want 3", cur)
	}
	// ListApplied should be sorted ascending
	list, err := st.ListApplied()
	if err != nil {
		t.Fatalf("ListApplied err: %v", err)
	}
	want := []int{1, 2, 3}
	if len(list) != len(want) {
		t.Fatalf("ListApplied length=%d, want %d; list=%v", len(list), len(want), list)
	}
	for i := range want {
		if list[i] != want[i] {
			t.Fatalf("ListApplied[%d]=%d, want %d (full: %v)", i, list[i], want[i], list)
		}
	}
}

func TestRemoveAndSetVersion(t *testing.T) {
	st := openTempStore(t)
	for _, v := range []int{1, 2, 3} {
		if err := st.Apply(v); err != nil {
			t.Fatalf("Apply(%d) err: %v", v, err)
		}
	}
	// Remove a specific version
	if err := st.Remove(2); err != nil {
		t.Fatalf("Remove(2) err: %v", err)
	}
	list, err := st.ListApplied()
	if err != nil {
		t.Fatalf("ListApplied err: %v", err)
	}
	if len(list) != 2 || list[0] != 1 || list[1] != 3 {
		t.Fatalf("after Remove, list=%v; want [1 3]", list)
	}
	// SetVersion to same current (3) on current 3 should be no-op
	cur, _ := st.CurrentVersion()
	if err := st.SetVersion(cur); err != nil {
		t.Fatalf("SetVersion(same=%d) err: %v", cur, err)
	}
	// Move down to 1 should delete >1
	if err := st.SetVersion(1); err != nil {
		t.Fatalf("SetVersion(1) err: %v", err)
	}
	list, err = st.ListApplied()
	if err != nil {
		t.Fatalf("ListApplied err: %v", err)
	}
	if len(list) != 1 || list[0] != 1 {
		t.Fatalf("after SetVersion(1), list=%v; want [1]", list)
	}
	// Attempt to move up should error
	if err := st.SetVersion(2); err == nil {
		t.Fatalf("expected error on SetVersion moving up, got nil")
	}
}

func TestCloseNilSafety(t *testing.T) {
	var s *Store
	if err := s.Close(); err != nil {
		t.Fatalf("nil Close should return nil, got %v", err)
	}
}

// Verify SQLite tables created by goose migrations exist
func TestSQLite_TablesExist(t *testing.T) {
	st := openTempStore(t)
	// ensure schema idempotent
	if err := st.EnsureSchema(); err != nil {
		t.Fatalf("EnsureSchema: %v", err)
	}
	// Check sqlite_master for table names (no goose table anymore)
	mustHave := []string{"schema_migrations", "migration_runs", "stored_env"}
	for _, tbl := range mustHave {
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

func TestStoredEnv_CRUD(t *testing.T) {
	st := openTempStore(t)
	// insert
	in := map[string]string{"rid": "123", "user": "alice"}
	if err := st.InsertStoredEnv(1, in); err != nil {
		t.Fatalf("InsertStoredEnv: %v", err)
	}
	// load
	m, err := st.LoadStoredEnv(1)
	if err != nil {
		t.Fatalf("LoadStoredEnv: %v", err)
	}
	if len(m) != 2 || m["rid"] != "123" || m["user"] != "alice" {
		t.Fatalf("unexpected stored env: %#v", m)
	}
	// update same version+name should replace
	if err := st.InsertStoredEnv(1, map[string]string{"rid": "999"}); err != nil {
		t.Fatalf("InsertStoredEnv replace: %v", err)
	}
	m, _ = st.LoadStoredEnv(1)
	if m["rid"] != "999" {
		t.Fatalf("expected rid=999 after replace, got %#v", m)
	}
	// delete
	if err := st.DeleteStoredEnv(1); err != nil {
		t.Fatalf("DeleteStoredEnv: %v", err)
	}
	m, _ = st.LoadStoredEnv(1)
	if len(m) != 0 {
		t.Fatalf("expected empty after delete, got %#v", m)
	}
}

// Test conv() converts '?' placeholders into $1, $2... when isPostgres is true
func TestConv_Postgres(t *testing.T) {
	st := &Store{Driver: DriverPostgresql}
	in := "INSERT INTO t(a,b,c) VALUES(?, ?, ?)"
	exp := "INSERT INTO t(a,b,c) VALUES($1, $2, $3)"
	if got := st.conv(in); got != exp {
		t.Fatalf("conv mismatch: got %q want %q", got, exp)
	}
	// non-postgres must pass-through
	st2 := &Store{Driver: DriverSqlite}
	if got := st2.conv(in); got != in {
		t.Fatalf("conv (sqlite) changed input: got %q want %q", got, in)
	}
}

// Test safe table names fallback to defaults when invalid identifiers are set
func TestSafeTableNames_FallbackOnInvalid(t *testing.T) {
	st := &Store{}
	// Set invalid names (contain dots/quotes)
	st.SetTableNames("public.migs", "migration-runs", "stored env")
	// Now ask for the safe names (should fallback to defaults)
	tn := st.safeTableNames()
	def := defaultTableNames()
	if tn.SchemaMigrations != def.SchemaMigrations || tn.MigrationRuns != def.MigrationRuns || tn.StoredEnv != def.StoredEnv {
		t.Fatalf("expected defaults on invalid names, got %+v want %+v", tn, def)
	}
	// Valid names should be preserved
	st.SetTableNames("app_schema", "app_runs", "app_env")
	tn2 := st.safeTableNames()
	if tn2.SchemaMigrations != "app_schema" || tn2.MigrationRuns != "app_runs" || tn2.StoredEnv != "app_env" {
		t.Fatalf("valid names not preserved: %+v", tn2)
	}
}

// Test RecordRun with env_json and LoadEnv returns that env
func TestRecordRunAndLoadEnv(t *testing.T) {
	st := openTempStore(t)
	// Record a run with an env map
	body := ""
	if err := st.RecordRun(1, "up", 200, &body, map[string]string{"a": "1", "b": "2"}, false); err != nil {
		t.Fatalf("RecordRun: %v", err)
	}
	m, err := st.LoadEnv(1, "up")
	if err != nil {
		t.Fatalf("LoadEnv: %v", err)
	}
	if len(m) != 2 || m["a"] != "1" || m["b"] != "2" {
		t.Fatalf("unexpected env: %+v", m)
	}
	// When no row exists, should return empty map and no error
	m2, err := st.LoadEnv(2, "up")
	if err != nil {
		t.Fatalf("LoadEnv (missing): %v", err)
	}
	if m2 == nil || len(m2) != 0 {
		t.Fatalf("expected empty map for missing env_json, got %+v", m2)
	}
	// Insert a malformed env_json row to ensure graceful fallback
	_, _ = st.DB.Exec("INSERT INTO migration_runs(version, direction, status_code, env_json, failed, ran_at) VALUES(?, ?, ?, ?, ?, ?)", 3, "up", 200, "not-json", 0, "2020-01-01T00:00:00Z")
	m3, err := st.LoadEnv(3, "up")
	if err != nil {
		t.Fatalf("LoadEnv (malformed): %v", err)
	}
	// Expect empty on malformed json
	if len(m3) != 0 {
		t.Fatalf("expected empty for malformed env_json, got %+v", m3)
	}
}

// Cover the unknown driver branch in Store.Connect
func TestStoreConnect_UnknownDriver(t *testing.T) {
	var st Store
	// Passing an unknown driver should return an error
	err := st.Connect(Config{Driver: "unknown-driver"})
	if err == nil {
		t.Fatalf("expected error for unknown driver")
	}
}

// Cover Close() path when connector is nil but DB is set (closes DB)
func TestStoreClose_DBOnly(t *testing.T) {
	// Open a lightweight in-memory sqlite DB directly and attach to Store
	db, err := sql.Open("sqlite", "file::memory:?cache=shared&_busy_timeout=5000&_fk=1")
	if err != nil {
		t.Fatalf("sql.Open sqlite: %v", err)
	}
	st := &Store{DB: db}
	if err := st.Close(); err != nil {
		t.Fatalf("Close(): %v", err)
	}
}

// Cover ToMap helpers used by connectors
func TestDriverConfig_ToMapHelpers(t *testing.T) {
	// Sqlite
	sc := &SqliteConfig{Path: "/tmp/x.db"}
	m := sc.ToMap()
	if m["path"] != "/tmp/x.db" {
		t.Fatalf("SqliteConfig.ToMap path mismatch: %#v", m)
	}
	// Postgres: build DSN from components when DSN empty
	pc := &PostgresConfig{Host: "h", Port: 0, User: "u", Password: "p", DBName: "d", SSLMode: ""}
	pm := pc.ToMap()
	dsn, _ := pm["dsn"].(string)
	if dsn == "" {
		t.Fatalf("PostgresConfig.ToMap should build DSN from components, got empty")
	}
	if dsn != "postgres://u:p@h:5432/d?sslmode=disable" {
		t.Fatalf("built DSN mismatch: %q", dsn)
	}
}

func TestSafeTableNames_DefaultsWhenEmpty(t *testing.T) {
	var s Store
	tn := s.safeTableNames()
	def := defaultTableNames()
	if tn != def {
		t.Fatalf("expected defaults when empty, got %+v", tn)
	}
}

func TestSafeTableNames_MixedValidity(t *testing.T) {
	var s Store
	// Only one valid provided, others invalid/empty -> fallback appropriately
	s.SetTableNames("valid_name", "", "invalid name with space")
	tn := s.safeTableNames()
	if tn.SchemaMigrations != "valid_name" {
		t.Fatalf("SchemaMigrations should keep valid_name, got %s", tn.SchemaMigrations)
	}
	def := defaultTableNames()
	if tn.MigrationRuns != def.MigrationRuns || tn.StoredEnv != def.StoredEnv {
		t.Fatalf("expected fallbacks for invalid/empty, got %+v", tn)
	}
}
