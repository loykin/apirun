package apimigrate

import (
	"context"
	"encoding/base64"
	"os"
	"path/filepath"
	"testing"
)

// Test that AcquireAuthAndSetEnv acquires a basic token and injects it into base env under _auth_token
func TestAcquireAuthAndSetEnv_Basic(t *testing.T) {
	ctx := context.Background()
	base := Env{Global: map[string]string{}}
	// basic: username/password -> base64(username:password)
	spec := map[string]interface{}{"username": "u", "password": "p"}
	v, err := AcquireAuthAndSetEnv(ctx, "basic", "example_basic", spec, &base)
	if err != nil {
		t.Fatalf("AcquireAuthAndSetEnv error: %v", err)
	}
	exp := base64.StdEncoding.EncodeToString([]byte("u:p"))
	if v != exp {
		t.Fatalf("unexpected token: got %q want %q", v, exp)
	}
	if base.Global[AuthTokenVar] != exp {
		t.Fatalf("_auth_token not injected: got %q want %q", base.Global[AuthTokenVar], exp)
	}
}

// Test OpenStoreFromOptions for sqlite default path and custom names
func TestOpenStoreFromOptions_SQLite_DefaultAndCustomNames(t *testing.T) {
	dir := t.TempDir()
	// Case 1: nil opts -> default sqlite file under dir
	st, err := OpenStoreFromOptions(dir, nil)
	if err != nil {
		t.Fatalf("OpenStoreFromOptions nil opts: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	// default file should exist
	if _, err := os.Stat(filepath.Join(dir, StoreDBFileName)); err != nil {
		t.Fatalf("expected default sqlite file, stat err: %v", err)
	}

	// Case 2: explicit SQLitePath and custom names
	customDir := t.TempDir()
	customDB := filepath.Join(customDir, "custom.db")
	opts := &StoreOptions{
		Backend:                 "sqlite",
		SQLitePath:              customDB,
		TableSchemaMigrations:   "app_schema",
		TableMigrationRuns:      "app_runs",
		TableStoredEnv:          "app_env",
		IndexStoredEnvByVersion: "app_idx",
	}
	st2, err := OpenStoreFromOptions(dir, opts)
	if err != nil {
		t.Fatalf("OpenStoreFromOptions custom: %v", err)
	}
	defer func() { _ = st2.Close() }()
	if _, err := os.Stat(customDB); err != nil {
		t.Fatalf("expected custom sqlite file at %s, stat err: %v", customDB, err)
	}
	// Verify custom tables exist in SQLite
	rows := st2.DB.QueryRow(`SELECT name FROM sqlite_master WHERE type='table' AND name='app_schema'`)
	var name string
	if err := rows.Scan(&name); err != nil || name != "app_schema" {
		t.Fatalf("expected custom table app_schema, got name=%q err=%v", name, err)
	}
}

// Test that postgres backend with empty DSN errors out
func TestOpenStoreFromOptions_Postgres_EmptyDSN_Err(t *testing.T) {
	_, err := OpenStoreFromOptions(t.TempDir(), &StoreOptions{Backend: "postgres", PostgresDSN: ""})
	if err == nil {
		t.Fatalf("expected error for empty PostgresDSN, got nil")
	}
}
