package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/loykin/apimigrate"
	"github.com/spf13/viper"
)

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
