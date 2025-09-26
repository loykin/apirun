package main

import (
	"database/sql"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/loykin/apirun"
	"github.com/spf13/viper"
)

// Dry-run up should not write to schema_migrations or migration_runs
func TestCLI_Up_DryRun_NoStoreMutations(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{"ok":true}`))
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
  save_response_body: true
migrate_dir: %s
`, tdir)
	cfgPath := writeFile(t, tdir, "config.yaml", cfg)

	v := viper.GetViper()
	v.Set("config", cfgPath)
	v.Set("v", false)
	v.Set("to", 0)
	v.Set("dry_run", true)
	v.Set("dry_run_from", 0)
	defer func() { v.Set("dry_run", false); v.Set("dry_run_from", 0) }()

	if err := upCmd.RunE(upCmd, nil); err != nil {
		t.Fatalf("up dry-run: %v", err)
	}
	// Validate that store file may not even exist; if it exists, tables should be empty
	dbPath := filepath.Join(tdir, apirun.StoreDBFileName)
	db, err := sql.Open("sqlite", "file:"+dbPath+"?_fk=1")
	if err == nil {
		defer func() { _ = db.Close() }()
		// Count rows in schema_migrations and migration_runs if tables exist
		for _, tbl := range []string{"schema_migrations", "migration_runs"} {
			_, _ = db.Exec("CREATE TABLE IF NOT EXISTS " + tbl + "(x TEXT)") // ensure queryable without failing
			row := db.QueryRow("SELECT COUNT(1) FROM " + tbl)
			var n int
			_ = row.Scan(&n)
			if n != 0 {
				t.Fatalf("expected 0 rows in %s in dry-run, got %d", tbl, n)
			}
		}
	}
}

// Dry-run down should not remove applied versions
func TestCLI_Down_DryRun_NoRemovals(t *testing.T) {
	// Prepare a real up run to create state
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{"ok":true}`))
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
migrate_dir: %s
`, tdir)
	cfgPath := writeFile(t, tdir, "config.yaml", cfg)

	v := viper.GetViper()
	v.Set("config", cfgPath)
	v.Set("v", false)
	v.Set("to", 0)
	v.Set("dry_run", false)
	if err := upCmd.RunE(upCmd, nil); err != nil {
		t.Fatalf("up real: %v", err)
	}

	// Now run down in dry-run mode
	v.Set("dry_run", true)
	v.Set("dry_run_from", 1)
	defer func() { v.Set("dry_run", false); v.Set("dry_run_from", 0) }()
	v.Set("to", 1)
	if err := downCmd.RunE(downCmd, nil); err != nil {
		t.Fatalf("down dry-run: %v", err)
	}

	// Ensure version 1 remains applied
	dbPath := filepath.Join(tdir, apirun.StoreDBFileName)
	db, err := sql.Open("sqlite", "file:"+dbPath+"?_fk=1")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer func() { _ = db.Close() }()
	row := db.QueryRow("SELECT COUNT(1) FROM schema_migrations WHERE version=1")
	var cnt int
	if err := row.Scan(&cnt); err == nil {
		if cnt != 1 {
			t.Fatalf("expected version 1 to remain applied, got %d rows", cnt)
		}
	}
}
