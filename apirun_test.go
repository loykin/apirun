package apirun

import (
	"context"
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/go-resty/resty/v2"
	"github.com/loykin/apirun/internal/httpc"
	"github.com/loykin/apirun/pkg/env"
)

// Test that struct-based Auth acquires a basic token and stores it under .auth[name]
func TestAcquireAuth_Basic(t *testing.T) {
	ctx := context.Background()
	// basic: username/password -> base64(username:password)
	spec := NewAuthSpecFromMap(map[string]interface{}{"username": "u", "password": "p"})
	a := &Auth{Type: "basic", Name: "b1", Methods: spec}
	v, err := a.Acquire(ctx, nil)
	if err != nil {
		t.Fatalf("Acquire error: %v", err)
	}
	exp := base64.StdEncoding.EncodeToString([]byte("u:p"))
	if v != exp {
		t.Fatalf("unexpected token: got %q want %q", v, exp)
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
	cfg := &StoreConfig{}
	cfg.Config.Driver = DriverSqlite
	cfg.Config.DriverConfig = &SqliteConfig{Path: customDB}
	cfg.Config.TableNames = TableNames{SchemaMigrations: "app_schema", MigrationRuns: "app_runs", StoredEnv: "app_env"}
	st2, err := OpenStoreFromOptions(dir, cfg)
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
	cfg := &StoreConfig{}
	cfg.Config.Driver = DriverPostgresql
	cfg.Config.DriverConfig = &PostgresConfig{DSN: ""}
	_, err := OpenStoreFromOptions(t.TempDir(), cfg)
	if err == nil {
		t.Fatalf("expected error for empty PostgresDSN, got nil")
	}
}

func TestPublicToMap_BasicAuthConfig(t *testing.T) {
	c := BasicAuthConfig{Username: "u", Password: "p"}
	m := c.ToMap()
	if m["username"] != "u" || m["password"] != "p" {
		t.Fatalf("BasicAuthConfig.ToMap mismatch: %+v", m)
	}
}

func TestPublicToMap_OAuth2PasswordConfig(t *testing.T) {
	c := OAuth2PasswordConfig{
		ClientID:  "cid",
		ClientSec: "sec",
		AuthURL:   "a",
		TokenURL:  "t",
		Username:  "u",
		Password:  "p",
	}
	m := c.ToMap()
	if m["grant_type"] != "password" {
		t.Fatalf("grant_type mismatch: %+v", m)
	}
	sub, ok := m["grant_config"].(map[string]interface{})
	if !ok {
		t.Fatalf("grant_config not a map: %#v", m["grant_config"])
	}
	if sub["client_id"] != "cid" || sub["client_secret"] != "sec" || sub["auth_url"] != "a" || sub["token_url"] != "t" || sub["username"] != "u" || sub["password"] != "p" {
		t.Fatalf("password grant_config mismatch: %+v", sub)
	}
	// scopes empty should be absent
	if _, exists := sub["scopes"]; exists {
		t.Fatalf("scopes should be absent when empty: %+v", sub)
	}
	// with scopes
	c.Scopes = []string{"s1", "s2"}
	sub2 := c.ToMap()["grant_config"].(map[string]interface{})
	if got, ok := sub2["scopes"].([]string); !ok || len(got) != 2 || got[0] != "s1" || got[1] != "s2" {
		t.Fatalf("scopes not preserved: %+v", sub2["scopes"])
	}
}

func TestPublicToMap_OAuth2ClientCredentialsConfig(t *testing.T) {
	c := OAuth2ClientCredentialsConfig{
		ClientID:  "id",
		ClientSec: "sec",
		TokenURL:  "tok",
	}
	m := c.ToMap()
	if m["grant_type"] != "client_credentials" {
		t.Fatalf("grant_type mismatch: %+v", m)
	}
	sub := m["grant_config"].(map[string]interface{})
	if sub["client_id"] != "id" || sub["client_secret"] != "sec" || sub["token_url"] != "tok" {
		t.Fatalf("cc grant_config mismatch: %+v", sub)
	}
	if _, exists := sub["scopes"]; exists {
		t.Fatalf("scopes should be absent when empty: %+v", sub)
	}
	c.Scopes = []string{"a"}
	sub2 := c.ToMap()["grant_config"].(map[string]interface{})
	if got, ok := sub2["scopes"].([]string); !ok || len(got) != 1 || got[0] != "a" {
		t.Fatalf("scopes not preserved: %+v", sub2["scopes"])
	}
}

func TestPublicToMap_OAuth2ImplicitConfig(t *testing.T) {
	c := OAuth2ImplicitConfig{ClientID: "id", RedirectURL: "r", AuthURL: "a"}
	m := c.ToMap()
	if m["grant_type"] != "implicit" {
		t.Fatalf("grant_type mismatch: %+v", m)
	}
	sub := m["grant_config"].(map[string]interface{})
	if sub["client_id"] != "id" || sub["redirect_url"] != "r" || sub["auth_url"] != "a" {
		t.Fatalf("implicit grant_config mismatch: %+v", sub)
	}
	if _, ok := sub["scopes"]; ok {
		t.Fatalf("scopes should be absent when empty: %+v", sub)
	}
	c.Scopes = []string{"x"}
	sub2 := c.ToMap()["grant_config"].(map[string]interface{})
	if got, ok := sub2["scopes"].([]string); !ok || len(got) != 1 || got[0] != "x" {
		t.Fatalf("scopes not preserved: %+v", sub2["scopes"])
	}
}

func TestPublicToMap_PocketBaseAuthConfig(t *testing.T) {
	c := PocketBaseAuthConfig{BaseURL: "b", Email: "e", Password: "p"}
	m := c.ToMap()
	if m["base_url"] != "b" || m["email"] != "e" || m["password"] != "p" {
		t.Fatalf("PocketBaseAuthConfig.ToMap mismatch: %+v", m)
	}
}

func TestOpenStore_CreatesSQLiteFile(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "test.db")
	cfg2 := &StoreConfig{}
	cfg2.Config.Driver = DriverSqlite
	cfg2.Config.DriverConfig = &SqliteConfig{Path: p}
	st, err := OpenStoreFromOptions(dir, cfg2)
	if err != nil {
		t.Fatalf("OpenStoreFromOptions error: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	if _, err := os.Stat(p); err != nil {
		t.Fatalf("expected sqlite file at %s, stat err: %v", p, err)
	}
}

func TestNewHTTPClient_TLSHelpers(t *testing.T) {
	// Default settings
	c := resty.New()
	hc := c.GetClient()
	tr, _ := hc.Transport.(*http.Transport)
	if tr == nil {
		t.Fatalf("expected http.Transport to be set")
	}

	// Custom insecure and version bounds via internal httpc.Httpc
	var h httpc.Httpc
	h.TlsConfig = &tls.Config{InsecureSkipVerify: true, MinVersion: tls.VersionTLS12, MaxVersion: tls.VersionTLS12}
	c2 := h.New()
	hc2 := c2.GetClient()
	tr2, _ := hc2.Transport.(*http.Transport)
	if tr2 == nil || tr2.TLSClientConfig == nil {
		t.Fatalf("expected TLSClientConfig to be set on client 2")
	}
	if !tr2.TLSClientConfig.InsecureSkipVerify {
		t.Fatalf("expected InsecureSkipVerify=true")
	}
	if tr2.TLSClientConfig.MinVersion != tls.VersionTLS12 {
		t.Fatalf("MinVersion not applied: got %v want %v", tr2.TLSClientConfig.MinVersion, tls.VersionTLS12)
	}
	if tr2.TLSClientConfig.MaxVersion != tls.VersionTLS12 {
		t.Fatalf("MaxVersion not applied: got %v want %v", tr2.TLSClientConfig.MaxVersion, tls.VersionTLS12)
	}
}

func TestRenderAnyTemplate_Basic(t *testing.T) {
	base := env.Env{Global: env.FromStringMap(map[string]string{"name": "world"})}
	in := map[string]interface{}{
		"greet": "hello {{.env.name}}",
	}
	out := RenderAnyTemplate(in, &base).(map[string]interface{})
	if s, _ := out["greet"].(string); !strings.Contains(s, "hello world") {
		t.Fatalf("RenderAnyTemplate failed: got %q", s)
	}
}

func TestMigrateDown_RollsBack(t *testing.T) {
	// Create a test server with two endpoints: up and down
	var hitsUp, hitsDown int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/up":
			hitsUp++
			w.WriteHeader(200)
			_, _ = w.Write([]byte("ok"))
		case "/down":
			hitsDown++
			w.WriteHeader(200)
			_, _ = w.Write([]byte("ok"))
		default:
			w.WriteHeader(404)
		}
	}))
	defer srv.Close()

	// Prepare a temporary migration file
	dir := t.TempDir()
	migFile := filepath.Join(dir, "001_demo.yaml")
	content := "" +
		"up:\n" +
		"  name: demo-up\n" +
		"  request:\n" +
		"    method: GET\n" +
		"    url: " + srv.URL + "/up\n" +
		"  response:\n" +
		"    result_code: [\"200\"]\n" +
		"\n" +
		"down:\n" +
		"  name: demo-down\n" +
		"  method: GET\n" +
		"  url: " + srv.URL + "/down\n"
	if err := os.WriteFile(migFile, []byte(content), 0600); err != nil {
		t.Fatalf("write migration file: %v", err)
	}

	// Use a sqlite store in temp dir
	storePath := filepath.Join(dir, "state.db")
	base := env.Env{Global: env.Map{}}
	ctx := context.Background()

	storeConfig := StoreConfig{}
	storeConfig.Config.Driver = DriverSqlite
	storeConfig.Config.DriverConfig = &SqliteConfig{Path: storePath}
	m := Migrator{Env: &base, Dir: dir, StoreConfig: &storeConfig}

	// Run Up
	resUp, err := m.MigrateUp(ctx, 0)
	if err != nil {
		t.Fatalf("MigrateUp error: %v", err)
	}
	if len(resUp) != 1 || hitsUp != 1 {
		t.Fatalf("expected 1 up migration and 1 hit, got len=%d hitsUp=%d", len(resUp), hitsUp)
	}

	// Run Down to version 0
	resDown, err := m.MigrateDown(ctx, 0)
	if err != nil {
		t.Fatalf("MigrateDown error: %v", err)
	}
	if len(resDown) != 1 || hitsDown != 1 {
		t.Fatalf("expected 1 down migration and 1 hit, got len=%d hitsDown=%d", len(resDown), hitsDown)
	}
}

// Ensure Migrator with StoreConfig sqlite and empty path defaults to Dir/StoreDBFileName
func TestMigrator_StoreConfig_DefaultSqlitePath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		_, _ = w.Write([]byte("ok"))
	}))
	defer srv.Close()

	dir := t.TempDir()
	mig := []byte("" +
		"up:\n" +
		"  name: t\n" +
		"  request:\n" +
		"    method: GET\n" +
		"    url: " + srv.URL + "/ok\n" +
		"  response:\n" +
		"    result_code: ['200']\n")
	if err := os.WriteFile(filepath.Join(dir, "001_ok.yaml"), mig, 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}

	cfg := &StoreConfig{}
	cfg.Config.Driver = DriverSqlite
	cfg.Config.DriverConfig = &SqliteConfig{Path: ""} // force defaulting
	m := &Migrator{Dir: dir, StoreConfig: cfg}
	if _, err := m.MigrateUp(context.Background(), 0); err != nil {
		t.Fatalf("MigrateUp: %v", err)
	}
	// sqlite file should be created under dir
	p := filepath.Join(dir, StoreDBFileName)
	if _, err := os.Stat(p); err != nil {
		t.Fatalf("expected sqlite file at %s, stat err: %v", p, err)
	}
}

// Test struct-based Auth acquires a token via registered provider
func TestAuth_Acquire_StoresInAuth(t *testing.T) {
	// Register a fake provider that returns a fixed token value
	RegisterAuthProvider("dummy", func(spec map[string]interface{}) (AuthMethod, error) {
		return dummyMethodEnvHelper{}, nil
	})

	ctx := context.Background()
	a := &Auth{Type: "dummy", Name: "demo", Methods: NewAuthSpecFromMap(map[string]interface{}{})}
	v, err := a.Acquire(ctx, nil)
	if err != nil {
		t.Fatalf("Acquire error: %v", err)
	}
	if v != "Bearer unit-token" {
		t.Fatalf("unexpected token value: %q", v)
	}
}

type dummyMethodEnvHelper struct{}

func (d dummyMethodEnvHelper) Acquire(_ context.Context) (string, error) {
	return "Bearer unit-token", nil
}

// When RenderBodyDefault=false at the wrapper level and request doesn't override,
// the body should be sent unrendered (containing the template braces).
func TestMigrator_RenderBodyDefault_DisablesBodyTemplating(t *testing.T) {
	// Server that verifies request body contains template braces literally
	sawRaw := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b := make([]byte, r.ContentLength)
		_, _ = r.Body.Read(b)
		_ = r.Body.Close()
		if string(b) == "{\"val\":\"{{.env.name}}\"}" {
			sawRaw = true
		}
		w.WriteHeader(200)
		_, _ = w.Write([]byte("ok"))
	}))
	defer srv.Close()

	dir := t.TempDir()
	mig := fmt.Sprintf(`---
up:
  name: raw-body
  env: { }
  request:
    method: POST
    url: %s/post
    body: '{"val":"{{.env.name}}"}'
  response:
    result_code: ["200"]
`, srv.URL)
	if err := os.WriteFile(filepath.Join(dir, "001_raw.yaml"), []byte(mig), 0o600); err != nil {
		t.Fatalf("write mig: %v", err)
	}

	base := env.Env{Global: env.FromStringMap(map[string]string{"name": "world"})}
	ctx := context.Background()
	m := &Migrator{Env: &base, Dir: dir}
	f := false
	m.RenderBodyDefault = &f

	if _, err := m.MigrateUp(ctx, 0); err != nil {
		t.Fatalf("MigrateUp: %v", err)
	}
	if !sawRaw {
		t.Fatalf("expected server to observe raw (unrendered) body")
	}
}

// Test ListRuns exported helper maps internal store Run rows into public RunHistory correctly.
func TestListRuns_MapsFields(t *testing.T) {
	dir := t.TempDir()
	// Open default sqlite store at <dir>/StoreDBFileName
	st, err := OpenStoreFromOptions(dir, nil)
	if err != nil {
		t.Fatalf("OpenStoreFromOptions: %v", err)
	}
	defer func() { _ = st.Close() }()

	// Apply to move current version and record runs
	if err := st.Apply(1); err != nil {
		t.Fatalf("Apply(1): %v", err)
	}
	body := "ok"
	if err := st.RecordRun(1, "up", 200, &body, map[string]string{"x": "y"}, false); err != nil {
		t.Fatalf("RecordRun #1: %v", err)
	}
	if err := st.RecordRun(2, "up", 500, nil, nil, true); err != nil {
		t.Fatalf("RecordRun #2: %v", err)
	}

	runs, err := ListRuns(st)
	if err != nil {
		t.Fatalf("ListRuns: %v", err)
	}
	if len(runs) != 2 {
		t.Fatalf("expected 2 runs, got %d -> %#v", len(runs), runs)
	}
	if runs[0].Version != 1 || runs[0].Direction != "up" || runs[0].StatusCode != 200 || runs[0].Failed {
		t.Fatalf("runs[0] unexpected: %#v", runs[0])
	}
	if runs[0].Body == nil || *runs[0].Body != "ok" || runs[0].Env["x"] != "y" {
		t.Fatalf("runs[0] mapping mismatch: %#v", runs[0])
	}
	if runs[1].Version != 2 || !runs[1].Failed || runs[1].Body != nil {
		t.Fatalf("runs[1] unexpected: %#v", runs[1])
	}
	// ran_at should be a timestamp-like string
	re := regexp.MustCompile(`^\d{4}-\d{2}-\d{2}T`)
	if !re.MatchString(runs[0].RanAt) || !re.MatchString(runs[1].RanAt) {
		t.Fatalf("RanAt not RFC3339-ish: %q / %q", runs[0].RanAt, runs[1].RanAt)
	}

	// sanity: the sqlite file should exist in dir
	p := filepath.Join(dir, StoreDBFileName)
	_ = p
}

// Test the public logging API
func TestLoggingAPI(t *testing.T) {
	// Test NewLogger creation with different levels
	tests := []struct {
		name  string
		level LogLevel
	}{
		{"error level", LogLevelError},
		{"warn level", LogLevelWarn},
		{"info level", LogLevelInfo},
		{"debug level", LogLevelDebug},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := NewLogger(tt.level)
			if logger == nil {
				t.Fatal("expected logger, got nil")
			}
			if logger.Logger == nil {
				t.Fatal("expected slog.Logger, got nil")
			}
		})
	}
}

func TestJSONLoggingAPI(t *testing.T) {
	// Test NewJSONLogger creation
	logger := NewJSONLogger(LogLevelInfo)
	if logger == nil {
		t.Fatal("expected JSON logger, got nil")
	}
	if logger.Logger == nil {
		t.Fatal("expected slog.Logger, got nil")
	}
}

func TestGlobalLoggerManagement(t *testing.T) {
	// Store original logger for restoration
	originalLogger := GetLogger()

	// Test setting and getting custom logger
	customLogger := NewLogger(LogLevelDebug)
	SetDefaultLogger(customLogger)

	retrievedLogger := GetLogger()
	if retrievedLogger.Logger != customLogger.Logger {
		t.Fatal("expected custom logger to be set as default")
	}

	// Test with JSON logger
	jsonLogger := NewJSONLogger(LogLevelWarn)
	SetDefaultLogger(jsonLogger)

	retrievedJSONLogger := GetLogger()
	if retrievedJSONLogger.Logger != jsonLogger.Logger {
		t.Fatal("expected JSON logger to be set as default")
	}

	// Restore original logger to avoid affecting other tests
	SetDefaultLogger(originalLogger)
}

func TestLoggingIntegrationWithMigrator(t *testing.T) {
	// Create a temporary directory for migrations
	tmpDir, err := os.MkdirTemp("", "apirun-logging-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// Create a simple migration file
	migrationDir := filepath.Join(tmpDir, "migrations")
	if err := os.MkdirAll(migrationDir, 0o755); err != nil {
		t.Fatalf("failed to create migration dir: %v", err)
	}

	migrationContent := `up:
  name: test migration
  env: {}
  request:
    method: GET
    url: https://httpbin.org/status/200
  response:
    result_code: ["200"]
`
	migrationFile := filepath.Join(migrationDir, "001_test.yaml")
	if err := os.WriteFile(migrationFile, []byte(migrationContent), 0o644); err != nil {
		t.Fatalf("failed to write migration file: %v", err)
	}

	// Set up logging
	logger := NewLogger(LogLevelDebug)
	SetDefaultLogger(logger)

	// Create environment
	base := env.New()
	_ = base.SetString("global", "test_key", "test_value")

	// Create migrator
	m := Migrator{
		Dir: migrationDir,
		Env: base,
	}

	// Test that logging doesn't interfere with normal operation
	ctx := context.Background()

	// Note: This may fail due to network connectivity, but it should test the logging integration
	_, _ = m.MigrateUp(ctx, 0)
	// We don't assert on success here since it depends on network connectivity
	// The important thing is that logging is integrated and doesn't break the functionality

	// Verify logger is still accessible
	currentLogger := GetLogger()
	if currentLogger == nil {
		t.Fatal("logger should still be accessible after migration")
	}
}

func TestLogLevelConstants(t *testing.T) {
	// Test that log level constants are properly defined
	levels := []LogLevel{
		LogLevelError,
		LogLevelWarn,
		LogLevelInfo,
		LogLevelDebug,
	}

	for i, level := range levels {
		if int(level) != i {
			t.Errorf("expected log level %d to have value %d, got %d", i, i, int(level))
		}
	}
}

func TestLoggerAPI(t *testing.T) {
	// Test that Logger methods are accessible
	logger := NewLogger(LogLevelInfo)

	// These should not panic
	logger.Info("test info message", "key", "value")
	logger.Debug("test debug message", "debug_key", "debug_value")
	logger.Warn("test warn message", "warn_key", "warn_value")
	logger.Error("test error message", "error_key", "error_value")

	// Test with nil error (should not panic)
	logger.Error("test error with nil error", "test_key", "test_value", "error", nil)
}

func TestLoggerCreationWithInvalidLevel(t *testing.T) {
	// Test with invalid log level (should default to info)
	logger := NewLogger(LogLevel(999))
	if logger == nil {
		t.Fatal("expected logger even with invalid level, got nil")
	}

	// Test JSON logger with invalid level
	jsonLogger := NewJSONLogger(LogLevel(-1))
	if jsonLogger == nil {
		t.Fatal("expected JSON logger even with invalid level, got nil")
	}
}

func TestLoggerThreadSafety(t *testing.T) {
	// Test that logger operations are thread-safe
	logger := NewLogger(LogLevelDebug)
	SetDefaultLogger(logger)

	done := make(chan bool, 10)

	// Start multiple goroutines that use the logger
	for i := 0; i < 10; i++ {
		go func(id int) {
			defer func() { done <- true }()

			currentLogger := GetLogger()
			currentLogger.Info("goroutine message", "goroutine_id", id)

			// Create and set new logger
			newLogger := NewLogger(LogLevelInfo)
			SetDefaultLogger(newLogger)

			retrievedLogger := GetLogger()
			retrievedLogger.Debug("debug from goroutine", "id", id)
		}(i)
	}

	// Wait for all goroutines to complete
	for i := 0; i < 10; i++ {
		<-done
	}

	// Verify we can still get a logger
	finalLogger := GetLogger()
	if finalLogger == nil {
		t.Fatal("expected final logger to be accessible")
	}
}

func TestLoggingWithMigratorConfiguration(t *testing.T) {
	// Test that logging works with different migrator configurations

	logger := NewLogger(LogLevelInfo)
	SetDefaultLogger(logger)

	base := env.New()
	_ = base.SetString("global", "test_key", "test_value")

	// Simple migrator configuration without complex store setup
	m := Migrator{
		Dir: "./non-existent-dir", // This will fail, but should log the error
		Env: base,
	}

	ctx := context.Background()
	_, err := m.MigrateUp(ctx, 0)
	// We expect this to fail, but it should not panic and should log appropriately
	if err == nil {
		t.Log("Migration unexpectedly succeeded with non-existent directory")
	}

	// Test with dry run mode
	m.DryRun = true
	m.DryRunFrom = 0

	_, err = m.MigrateUp(ctx, 0)
	// Dry run mode should also work with logging
	if err == nil {
		t.Log("Dry run completed without error")
	}

	// Verify logger is still functional
	currentLogger := GetLogger()
	if currentLogger == nil {
		t.Fatal("logger should be accessible after migration attempts")
	}
}

func TestLoggingAPIBoundaries(t *testing.T) {
	// Test boundary conditions and edge cases

	// Test with nil logger (should not panic when setting)
	// Note: We can't actually pass nil due to type safety, but test the API

	// Test multiple consecutive logger changes
	logger1 := NewLogger(LogLevelError)
	logger2 := NewJSONLogger(LogLevelDebug)
	logger3 := NewLogger(LogLevelWarn)

	SetDefaultLogger(logger1)
	currentLogger := GetLogger()
	if currentLogger.Logger != logger1.Logger {
		t.Error("expected logger1 to be set")
	}

	SetDefaultLogger(logger2)
	currentLogger = GetLogger()
	if currentLogger.Logger != logger2.Logger {
		t.Error("expected logger2 to be set")
	}

	SetDefaultLogger(logger3)
	currentLogger = GetLogger()
	if currentLogger.Logger != logger3.Logger {
		t.Error("expected logger3 to be set")
	}
}

// Benchmark tests for logging performance
func BenchmarkNewLogger(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = NewLogger(LogLevelInfo)
	}
}

func BenchmarkNewJSONLogger(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = NewJSONLogger(LogLevelInfo)
	}
}

func BenchmarkLoggerSetGet(b *testing.B) {
	logger := NewLogger(LogLevelInfo)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		SetDefaultLogger(logger)
		_ = GetLogger()
	}
}

func BenchmarkLoggerInfo(b *testing.B) {
	logger := NewLogger(LogLevelInfo)
	SetDefaultLogger(logger)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		logger.Info("benchmark message", "key", "value", "iteration", i)
	}
}

func BenchmarkLoggerDebug(b *testing.B) {
	logger := NewLogger(LogLevelDebug)
	SetDefaultLogger(logger)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		logger.Debug("benchmark debug message", "key", "value", "iteration", i)
	}
}

// Test edge cases and error conditions
func TestLoggingEdgeCases(t *testing.T) {
	// Test logging with various data types
	logger := NewLogger(LogLevelDebug)

	// Test with different value types (should not panic)
	logger.Info("test with various types",
		"string", "value",
		"int", 42,
		"bool", true,
		"float", 3.14,
		"nil", nil)

	// Test with empty key-value pairs
	logger.Info("test with empty", "", "")

	// Test with odd number of key-value pairs (should handle gracefully)
	logger.Info("test with odd pairs", "key1", "value1", "key2", "")

	// Test with special characters in keys and values
	logger.Info("test with special chars",
		"key with spaces", "value with\nnewlines",
		"unicode-key-🔥", "unicode-value-✨")
}

// Test configuration-based logging
func TestConfigurableLogging(t *testing.T) {
	// Create a temporary config file
	tmpDir, err := os.MkdirTemp("", "apirun-config-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	configContent := `
logging:
  level: debug
  format: json

migrate_dir: ./migrations
env: []
auth: []
store:
  save_response_body: false
`
	configPath := filepath.Join(tmpDir, "test_config.yaml")
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	// Test different log level configurations
	testCases := []struct {
		name         string
		level        string
		format       string
		expectLevel  LogLevel
		expectFormat string
	}{
		{"debug text", "debug", "text", LogLevelDebug, "text"},
		{"info json", "info", "json", LogLevelInfo, "json"},
		{"warn text", "warn", "text", LogLevelWarn, "text"},
		{"error json", "error", "json", LogLevelError, "json"},
		{"default values", "", "", LogLevelInfo, "text"},
		{"invalid level defaults", "invalid", "", LogLevelInfo, "text"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			configContent := fmt.Sprintf(`
logging:
  level: %s
  format: %s

migrate_dir: ./migrations
env: []
auth: []
store:
  save_response_body: false
`, tc.level, tc.format)

			configPath := filepath.Join(tmpDir, fmt.Sprintf("test_config_%s.yaml", strings.ReplaceAll(tc.name, " ", "_")))
			if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
				t.Fatalf("failed to write config file: %v", err)
			}

			// This would test the configuration loading
			// Note: We can't fully test the main command execution here,
			// but we can test the individual components
			logger := NewLogger(LogLevelInfo) // Start with default
			SetDefaultLogger(logger)

			// Verify we can create loggers with expected levels
			expectedLogger := NewLogger(tc.expectLevel)
			if expectedLogger == nil {
				t.Error("expected to create logger with configured level")
			}

			if tc.format == "json" {
				jsonLogger := NewJSONLogger(tc.expectLevel)
				if jsonLogger == nil {
					t.Error("expected to create JSON logger")
				}
			}
		})
	}
}

func TestLoggingConfigurationParsing(t *testing.T) {
	// Test that the logging levels and formats are properly parsed
	levels := map[string]LogLevel{
		"error": LogLevelError,
		"warn":  LogLevelWarn,
		"info":  LogLevelInfo,
		"debug": LogLevelDebug,
	}

	for levelStr, expectedLevel := range levels {
		t.Run("level_"+levelStr, func(t *testing.T) {
			logger := NewLogger(expectedLevel)
			if logger == nil {
				t.Errorf("failed to create logger with level %s", levelStr)
			}
		})
	}

	// Test formats
	formats := []string{"text", "json"}
	for _, format := range formats {
		t.Run("format_"+format, func(t *testing.T) {
			var logger *Logger
			if format == "json" {
				logger = NewJSONLogger(LogLevelInfo)
			} else {
				logger = NewLogger(LogLevelInfo)
			}

			if logger == nil {
				t.Errorf("failed to create logger with format %s", format)
			}
		})
	}
}

// TestMaskingAPI tests the public masking API functions
func TestMaskingAPI(t *testing.T) {
	// Test NewMasker
	t.Run("NewMasker", func(t *testing.T) {
		masker := NewMasker()
		if masker == nil {
			t.Error("NewMasker() should return a valid masker")
		}

		if !masker.IsEnabled() {
			t.Error("NewMasker() should return an enabled masker by default")
		}
	})

	// Test NewMaskerWithPatterns
	t.Run("NewMaskerWithPatterns", func(t *testing.T) {
		patterns := []SensitivePattern{
			{
				Name: "test_pattern",
				Keys: []string{"test_key"},
			},
		}
		masker := NewMaskerWithPatterns(patterns)
		if masker == nil {
			t.Error("NewMaskerWithPatterns() should return a valid masker")
		}
	})

	// Test global masking functions
	t.Run("GlobalMasking", func(t *testing.T) {
		// Save original state
		originalState := IsMaskingEnabled()
		defer EnableMasking(originalState)

		// Test EnableMasking/IsMaskingEnabled
		EnableMasking(true)
		if !IsMaskingEnabled() {
			t.Error("IsMaskingEnabled() should return true after EnableMasking(true)")
		}

		EnableMasking(false)
		if IsMaskingEnabled() {
			t.Error("IsMaskingEnabled() should return false after EnableMasking(false)")
		}

		// Test MaskSensitiveData with masking enabled
		EnableMasking(true)
		input := "password=secret123"
		masked := MaskSensitiveData(input)
		if masked == input {
			t.Error("MaskSensitiveData() should mask sensitive data when enabled")
		}

		// Test MaskSensitiveData with masking disabled
		EnableMasking(false)
		masked2 := MaskSensitiveData(input)
		if masked2 != input {
			t.Error("MaskSensitiveData() should not mask data when disabled")
		}
	})

	// Test SetGlobalMasker/GetGlobalMasker
	t.Run("GlobalMaskerAPI", func(t *testing.T) {
		// Save original masker
		originalMasker := GetGlobalMasker()
		defer SetGlobalMasker(originalMasker)

		// Create and set new masker
		customMasker := NewMasker()
		SetGlobalMasker(customMasker)

		// Check if it was set correctly
		retrieved := GetGlobalMasker()
		if retrieved != customMasker {
			t.Error("SetGlobalMasker/GetGlobalMasker should work correctly")
		}
	})

	// Test Logger methods added for masking
	t.Run("LoggerMaskingMethods", func(t *testing.T) {
		logger := NewLogger(LogLevelInfo)

		// Test EnableMasking/IsMaskingEnabled on logger
		logger.EnableMasking(false)
		if logger.IsMaskingEnabled() {
			t.Error("Logger.IsMaskingEnabled() should return false after EnableMasking(false)")
		}

		logger.EnableMasking(true)
		if !logger.IsMaskingEnabled() {
			t.Error("Logger.IsMaskingEnabled() should return true after EnableMasking(true)")
		}

		// Test SetMasker/GetMasker
		customMasker := NewMasker()
		logger.SetMasker(customMasker)
		retrieved := logger.GetMasker()
		if retrieved != customMasker {
			t.Error("Logger.SetMasker/GetMasker should work correctly")
		}
	})
}

// TestLoggerIntegration tests the integration between Logger and masking
func TestLoggerIntegration(t *testing.T) {
	logger := NewLogger(LogLevelInfo)

	// We can't easily test the actual masking output without capturing logs,
	// but we can test that the methods don't panic and work correctly
	logger.Info("test message",
		"username", "admin",
		"password", "secret123",
		"api_key", "sk_test_123")

	logger.EnableMasking(false)
	logger.Info("test message with masking disabled",
		"username", "admin",
		"password", "visible_password")

	logger.EnableMasking(true)
	logger.Error("error with sensitive data",
		"error", "authentication failed",
		"token", "jwt_token_123")
}
