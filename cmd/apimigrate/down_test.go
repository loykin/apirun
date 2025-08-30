package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/loykin/apimigrate"
	"github.com/spf13/viper"
)

// Note: helper functions writeFile and basicVal are defined in up_test.go in the same package.

// Build two migrations with down steps that use different auth tokens and
// verify that running `down` to 0 triggers both downs with the correct headers.
func TestDownCmd_FullRollback_AuthChanges(t *testing.T) {
	calls := make(map[string]int)

	// Expect Authorization headers to use the last acquired token for both downs (reverted behavior)
	exp1 := basicVal("u2", "p2")
	exp2 := basicVal("u2", "p2")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// We focus assertions on down endpoints only
		switch r.URL.Path {
		case "/down1":
			if r.Method != http.MethodDelete {
				t.Fatalf("/down1 expected DELETE, got %s", r.Method)
			}
			if got := r.Header.Get("Authorization"); got != exp1 {
				t.Fatalf("/down1 expected Authorization %q, got %q", exp1, got)
			}
			calls["/down1"]++
		case "/down2":
			if r.Method != http.MethodDelete {
				t.Fatalf("/down2 expected DELETE, got %s", r.Method)
			}
			if got := r.Header.Get("Authorization"); got != exp2 {
				t.Fatalf("/down2 expected Authorization %q, got %q", exp2, got)
			}
			calls["/down2"]++
		default:
			// Up endpoints or others: accept and ignore
		}
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	tdir := t.TempDir()

	// Two versioned migrations.
	// Up sections can be simple GETs (we don't assert them here).
	// Down sections perform DELETE to paths /down1 and /down2 using different auth names.
	m1 := fmt.Sprintf(`---
up:
  name: v1
  request:
    method: GET
    url: %s/up1
  response:
    result_code: ["200"]
down:
  name: v1down
  auth: a1
  method: DELETE
  url: %s/down1
  headers:
    - name: Authorization
      value: "Basic {{._auth_token}}"
`, srv.URL, srv.URL)
	m2 := fmt.Sprintf(`---
up:
  name: v2
  request:
    method: GET
    url: %s/up2
  response:
    result_code: ["200"]
down:
  name: v2down
  auth: a2
  method: DELETE
  url: %s/down2
  headers:
    - name: Authorization
      value: "Basic {{._auth_token}}"
`, srv.URL, srv.URL)
	_ = writeFile(t, tdir, "001_first.yaml", m1)
	_ = writeFile(t, tdir, "002_second.yaml", m2)

	// Config with two basic providers (new top-level auth array schema)
	cfg := fmt.Sprintf(`---
auth:
  - type: basic
    name: a1
    config:
      username: u1
      password: p1
  - type: basic
    name: a2
    config:
      username: u2
      password: p2
migrate_dir: %s
`, tdir)
	cfgPath := writeFile(t, tdir, "config.yaml", cfg)

	v := viper.GetViper()
	v.Set("config", cfgPath)
	v.Set("v", false)
	v.Set("to", 0)

	// First, apply up to record both versions
	if err := upCmd.RunE(upCmd, nil); err != nil {
		t.Fatalf("upCmd.RunE error: %v", err)
	}
	// Then rollback all the way down to 0
	if err := downCmd.RunE(downCmd, nil); err != nil {
		t.Fatalf("downCmd.RunE error: %v", err)
	}

	if calls["/down1"] != 1 || calls["/down2"] != 1 {
		t.Fatalf("expected one call each to /down1 and /down2, got: %v", calls)
	}
}

// Test partial rollback: after applying two versions, roll back to version 1.
// Only the highest version (2) should be rolled back and thus only /down2 should be called.
func TestDownCmd_PartialRollback_ToVersion(t *testing.T) {
	calls := make(map[string]int)
	exp2 := basicVal("uu2", "pp2")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/down2":
			if r.Method != http.MethodDelete {
				t.Fatalf("/down2 expected DELETE, got %s", r.Method)
			}
			if got := r.Header.Get("Authorization"); got != exp2 {
				t.Fatalf("/down2 expected Authorization %q, got %q", exp2, got)
			}
			calls["/down2"]++
		}
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
    url: %s/up1
  response:
    result_code: ["200"]
down:
  name: v1down
  auth: aa1
  method: DELETE
  url: %s/down1
  headers:
    - name: Authorization
      value: "Basic {{._auth_token}}"
`, srv.URL, srv.URL)
	m2 := fmt.Sprintf(`---
up:
  name: v2
  request:
    method: GET
    url: %s/up2
  response:
    result_code: ["200"]
down:
  name: v2down
  auth: aa2
  method: DELETE
  url: %s/down2
  headers:
    - name: Authorization
      value: "Basic {{._auth_token}}"
`, srv.URL, srv.URL)
	_ = writeFile(t, tdir, "001_first.yaml", m1)
	_ = writeFile(t, tdir, "002_second.yaml", m2)

	cfg := fmt.Sprintf(`---
auth:
  - type: basic
    name: aa1
    config:
      username: uu1
      password: pp1
  - type: basic
    name: aa2
    config:
      username: uu2
      password: pp2
migrate_dir: %s
`, tdir)
	cfgPath := writeFile(t, tdir, "config.yaml", cfg)

	v := viper.GetViper()
	v.Set("config", cfgPath)
	v.Set("v", false)

	// Apply both
	v.Set("to", 0)
	if err := upCmd.RunE(upCmd, nil); err != nil {
		t.Fatalf("upCmd.RunE error: %v", err)
	}

	// Now roll back to version 1
	v.Set("to", 1)
	if err := downCmd.RunE(downCmd, nil); err != nil {
		t.Fatalf("downCmd.RunE error: %v", err)
	}

	if calls["/down2"] != 1 {
		t.Fatalf("expected one call to /down2 only, got: %v", calls)
	}

	// Verify store current version is 1
	dbPath := filepath.Join(tdir, apimigrate.StoreDBFileName)
	st, err := apimigrate.OpenStore(dbPath)
	if err != nil {
		t.Fatalf("OpenStore error: %v", err)
	}
	defer func() { _ = st.Close() }()
	cur, err := st.CurrentVersion()
	if err != nil {
		t.Fatalf("CurrentVersion error: %v", err)
	}
	if cur != 1 {
		t.Fatalf("expected current version 1 after partial down, got %d", cur)
	}
}
