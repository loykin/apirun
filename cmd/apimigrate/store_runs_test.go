package main

import (
	"database/sql"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
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
