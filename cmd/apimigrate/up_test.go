package main

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/loykin/apimigrate/internal/auth"
	"github.com/spf13/viper"
)

func writeFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
	return p
}

func basicVal(u, p string) string {
	return "Basic " + base64.StdEncoding.EncodeToString([]byte(u+":"+p))
}

// Test that running `up` applies multiple migrations that each use a different auth_name
// and that the correct Authorization header is sent for each request when auth is provided
// via the new top-level auth array in config.
func TestUpCmd_AuthChanges_WithTopLevelAuthArray(t *testing.T) {
	auth.ClearTokens()
	// Prepare expectations
	exp1 := basicVal("u1", "p1")
	exp2 := basicVal("u2", "p2")
	calls := make(map[string]int)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		path := r.URL.Path
		calls[path]++
		switch path {
		case "/one":
			if got := r.Header.Get("Authorization"); got != exp1 {
				t.Fatalf("/one expected Authorization %q, got %q", exp1, got)
			}
		case "/two":
			if got := r.Header.Get("Authorization"); got != exp2 {
				t.Fatalf("/two expected Authorization %q, got %q", exp2, got)
			}
		default:
			// unexpected
		}
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	tdir := t.TempDir()
	// Two migrations, each targeting a different path and using different auth_name
	m1 := fmt.Sprintf(`---
up:
  name: first
  request:
    method: GET
    url: %s/one
    auth_name: a1
  response:
    result_code: ["200"]
`, srv.URL)
	m2 := fmt.Sprintf(`---
up:
  name: second
  request:
    method: GET
    url: %s/two
    auth_name: a2
  response:
    result_code: ["200"]
`, srv.URL)
	_ = writeFile(t, tdir, "001_first.yaml", m1)
	_ = writeFile(t, tdir, "002_second.yaml", m2)

	// Config with top-level auth array defining both providers (new schema requires nested config)
	cfg := fmt.Sprintf(`---
auth:
  - type: basic
    config:
      name: a1
      username: u1
      password: p1
  - type: basic
    config:
      name: a2
      username: u2
      password: p2
migrate_dir: %s
`, tdir)
	cfgPath := writeFile(t, tdir, "config.yaml", cfg)

	// Configure Viper for the command
	v := viper.GetViper()
	v.Set("config", cfgPath)
	v.Set("v", false)
	v.Set("to", 0)

	// Run the up command
	if err := upCmd.RunE(upCmd, nil); err != nil {
		t.Fatalf("upCmd.RunE error: %v", err)
	}

	if calls["/one"] != 1 || calls["/two"] != 1 {
		t.Fatalf("expected one call to /one and /two, got: %v", calls)
	}
}

// Verify again with different provider names using the same top-level auth array schema.
func TestUpCmd_AuthChanges_WithTopLevelAuthArray_Variant(t *testing.T) {
	auth.ClearTokens()
	exp1 := basicVal("lu1", "lp1")
	exp2 := basicVal("lu2", "lp2")
	calls := make(map[string]int)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		path := r.URL.Path
		calls[path]++
		switch path {
		case "/one":
			if got := r.Header.Get("Authorization"); got != exp1 {
				t.Fatalf("/one expected Authorization %q, got %q", exp1, got)
			}
		case "/two":
			if got := r.Header.Get("Authorization"); got != exp2 {
				t.Fatalf("/two expected Authorization %q, got %q", exp2, got)
			}
		}
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	tdir := t.TempDir()
	m1 := fmt.Sprintf(`---
up:
  name: first
  request:
    method: GET
    url: %s/one
    auth_name: la1
  response:
    result_code: ["200"]
`, srv.URL)
	m2 := fmt.Sprintf(`---
up:
  name: second
  request:
    method: GET
    url: %s/two
    auth_name: la2
  response:
    result_code: ["200"]
`, srv.URL)
	_ = writeFile(t, tdir, "001_first.yaml", m1)
	_ = writeFile(t, tdir, "002_second.yaml", m2)

	cfg := fmt.Sprintf(`---
auth:
  - type: basic
    config:
      name: la1
      username: lu1
      password: lp1
  - type: basic
    config:
      name: la2
      username: lu2
      password: lp2
migrate_dir: %s
`, tdir)
	cfgPath := writeFile(t, tdir, "config.yaml", cfg)

	v := viper.GetViper()
	v.Set("config", cfgPath)
	v.Set("v", false)
	v.Set("to", 0)

	if err := upCmd.RunE(upCmd, nil); err != nil {
		t.Fatalf("upCmd.RunE error: %v", err)
	}
	if calls["/one"] != 1 || calls["/two"] != 1 {
		t.Fatalf("expected one call to /one and /two, got: %v", calls)
	}
}
