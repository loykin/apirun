package apimigrate

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
	"github.com/loykin/apimigrate/internal/httpc"
	"github.com/loykin/apimigrate/pkg/env"
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
	cfg.Config.Driver = DriverPostgres
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
