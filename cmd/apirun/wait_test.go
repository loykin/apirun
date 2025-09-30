package main

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/spf13/viper"
)

func TestWait_BasicPollingUntilAlive(t *testing.T) {
	var calls int32
	// Server returns 503 for the first 3 calls, then 200
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c := atomic.AddInt32(&calls, 1)
		if c <= 3 {
			w.WriteHeader(503)
			_, _ = w.Write([]byte("not ready"))
			return
		}
		w.WriteHeader(200)
		_, _ = w.Write([]byte("ok"))
	}))
	defer srv.Close()

	tdir := t.TempDir()
	cfg := fmt.Sprintf(`---
wait:
  url: %s/health
  method: GET
  status: 200
  timeout: 2s
  interval: 100ms
migrate_dir: %s
`, srv.URL, tdir)
	cfgPath := writeFile(t, tdir, "config.yaml", cfg)

	v := viper.GetViper()
	v.Set("config", cfgPath)
	v.Set("v", false)
	v.Set("to", 0)

	if err := upCmd.RunE(upCmd, nil); err != nil {
		t.Fatalf("upCmd.RunE error: %v", err)
	}
	if atomic.LoadInt32(&calls) < 4 {
		t.Fatalf("expected at least 4 calls (3 failures + 1 success), got %d", calls)
	}
}

func TestWait_DefaultsMethodAndStatus(t *testing.T) {
	var methodGot string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		methodGot = r.Method
		w.WriteHeader(200)
	}))
	defer srv.Close()
	// Omit method and status => defaults GET and 200
	tdir := t.TempDir()
	cfg := fmt.Sprintf(`---
wait:
  url: %s/health
  timeout: 1s
  interval: 50ms
migrate_dir: %s
`, srv.URL, tdir)
	cfgPath := writeFile(t, tdir, "config.yaml", cfg)
	v := viper.GetViper()
	v.Set("config", cfgPath)
	v.Set("v", false)
	v.Set("to", 0)
	if err := upCmd.RunE(upCmd, nil); err != nil {
		t.Fatalf("RunE error: %v", err)
	}
	if methodGot != http.MethodGet {
		t.Fatalf("expected default method GET, got %s", methodGot)
	}
}

func TestWait_HEADMethod(t *testing.T) {
	var gotHEAD int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodHead {
			atomic.AddInt32(&gotHEAD, 1)
		}
		w.WriteHeader(200)
	}))
	defer srv.Close()
	tdir := t.TempDir()
	cfg := fmt.Sprintf(`---
wait:
  url: %s/health
  method: HEAD
  status: 200
  timeout: 1s
  interval: 50ms
migrate_dir: %s
`, srv.URL, tdir)
	cfgPath := writeFile(t, tdir, "config.yaml", cfg)
	v := viper.GetViper()
	v.Set("config", cfgPath)
	v.Set("v", false)
	v.Set("to", 0)
	if err := upCmd.RunE(upCmd, nil); err != nil {
		t.Fatalf("RunE error: %v", err)
	}
	if atomic.LoadInt32(&gotHEAD) == 0 {
		t.Fatalf("expected at least one HEAD request")
	}
}

func TestWait_TimeoutErrorIncludesLastStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(503)
	}))
	defer srv.Close()
	tdir := t.TempDir()
	cfg := fmt.Sprintf(`---
wait:
  url: %s/health
  method: GET
  status: 200
  timeout: 300ms
  interval: 100ms
migrate_dir: %s
`, srv.URL, tdir)
	cfgPath := writeFile(t, tdir, "config.yaml", cfg)
	v := viper.GetViper()
	v.Set("config", cfgPath)
	v.Set("v", false)
	v.Set("to", 0)
	err := upCmd.RunE(upCmd, nil)
	if err == nil {
		t.Fatalf("expected timeout error, got nil")
	}
	if !strings.Contains(err.Error(), "last=503") {
		t.Fatalf("expected error to include last=503, got %v", err)
	}
}

func TestWait_TemplatedURL(t *testing.T) {
	var seen int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&seen, 1)
		w.WriteHeader(200)
	}))
	defer srv.Close()
	tdir := t.TempDir()
	cfg := fmt.Sprintf(`---
env:
  - name: base
    value: %s
wait:
  url: "{{.env.base}}/health"
  timeout: 1s
  interval: 50ms
migrate_dir: %s
`, srv.URL, tdir)
	cfgPath := writeFile(t, tdir, "config.yaml", cfg)
	v := viper.GetViper()
	v.Set("config", cfgPath)
	v.Set("v", false)
	v.Set("to", 0)
	if err := upCmd.RunE(upCmd, nil); err != nil {
		t.Fatalf("RunE error: %v", err)
	}
	if atomic.LoadInt32(&seen) == 0 {
		t.Fatalf("expected server to be hit at least once")
	}
}

func TestParseTLSVersion(t *testing.T) {
	tests := []struct {
		input    string
		expected uint16
	}{
		// TLS 1.0 variants
		{"1.0", tls.VersionTLS10},
		{"10", tls.VersionTLS10},
		{"tls1.0", tls.VersionTLS10},
		{"tls10", tls.VersionTLS10},
		{"TLS1.0", tls.VersionTLS10},
		{" 1.0 ", tls.VersionTLS10}, // with whitespace

		// TLS 1.1 variants
		{"1.1", tls.VersionTLS11},
		{"11", tls.VersionTLS11},
		{"tls1.1", tls.VersionTLS11},
		{"tls11", tls.VersionTLS11},

		// TLS 1.2 variants
		{"1.2", tls.VersionTLS12},
		{"12", tls.VersionTLS12},
		{"tls1.2", tls.VersionTLS12},
		{"tls12", tls.VersionTLS12},

		// TLS 1.3 variants
		{"1.3", tls.VersionTLS13},
		{"13", tls.VersionTLS13},
		{"tls1.3", tls.VersionTLS13},
		{"tls13", tls.VersionTLS13},

		// Invalid/empty cases
		{"", 0},
		{"invalid", 0},
		{"2.0", 0},
		{"tls2.0", 0},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("input_%s", tt.input), func(t *testing.T) {
			result := parseTLSVersion(tt.input)
			if result != tt.expected {
				t.Errorf("parseTLSVersion(%q) = %d, expected %d", tt.input, result, tt.expected)
			}
		})
	}
}
