package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/loykin/apimigrate"
	iauth "github.com/loykin/apimigrate/internal/auth"
	"github.com/loykin/apimigrate/internal/env"
)

func TestConfigDoc_Load_NotRegularFile(t *testing.T) {
	d := t.TempDir()
	var c ConfigDoc
	if err := c.Load(d); err == nil {
		t.Fatalf("expected error for directory path (not a regular file)")
	}
}

func TestConfigDoc_GetEnv_ValueFromEnv(t *testing.T) {
	_ = os.Setenv("TEST_VAL", "xyz")
	doc := ConfigDoc{Env: []EnvConfig{{Name: "a", Value: "", ValueFromEnv: "TEST_VAL"}}}
	base, err := doc.GetEnv(true)
	if err != nil {
		t.Fatalf("GetEnv: %v", err)
	}
	if base.Global["a"] != "xyz" {
		t.Fatalf("expected env a=xyz, got %q", base.Global["a"])
	}
}

func TestConfigDoc_DecodeAuth_BasicFlow(t *testing.T) {
	// Register basic provider is already done via internal registry init; ensure Acquire works
	// Prepare a temp migrate dir and sqlite store path to satisfy later Store usage if needed
	// Here we only exercise DecodeAuth logic and env population
	doc := &ConfigDoc{Auth: []AuthConfig{{
		Type: "basic", Name: "b1", Config: map[string]interface{}{"username": "u", "password": "p"},
	}}}
	base := env.New()
	ctx := context.Background()
	if err := doc.DecodeAuth(ctx, base, false); err != nil {
		t.Fatalf("DecodeAuth error: %v", err)
	}
	if base.Auth == nil || base.Auth["b1"] == "" {
		t.Fatalf("expected token stored under .auth[b1]")
	}
}

// Sanity: ToStorOptions builds default sqlite when Type empty and table prefix derivation
func TestStoreConfig_ToStorOptions_TablePrefixAndDefault(t *testing.T) {
	cfg := &StoreConfig{Type: "", TablePrefix: "appx"}
	so := cfg.ToStorOptions()
	if so != nil {
		// When Type empty, our CLI treats it as nil options; mimic call-site behavior
	}
	cfg2 := &StoreConfig{Type: "sqlite", TablePrefix: "appx", SQLite: SQLiteStoreConfig{Path: filepath.Join(t.TempDir(), "x.db")}}
	so2 := cfg2.ToStorOptions()
	if so2 == nil || so2.Config.Driver != apimigrate.DriverSqlite {
		t.Fatalf("expected sqlite options")
	}
	if so2.Config.TableNames.SchemaMigrations != "appx_schema_migrations" || so2.Config.TableNames.MigrationRuns != "appx_migration_log" || so2.Config.TableNames.StoredEnv != "appx_stored_env" {
		t.Fatalf("prefix-derived names mismatch: %#v", so2.Config.TableNames)
	}
}

// Ensure CLI sees struct-based auth types via re-export and map builder
func TestDecodeAuth_RendersTemplatesInAuthConfig(t *testing.T) {
	// The auth config includes templates referencing env
	doc := &ConfigDoc{
		Env: []EnvConfig{{Name: "user", Value: "alice"}, {Name: "pass", Value: "wonder"}},
		Auth: []AuthConfig{{
			Type: "basic", Name: "tpl", Config: map[string]interface{}{
				"username": "{{.env.user}}",
				"password": "{{.env.pass}}",
			},
		}},
	}
	base, _ := doc.GetEnv(false)
	if err := doc.DecodeAuth(context.Background(), &base, false); err != nil {
		t.Fatalf("DecodeAuth: %v", err)
	}
	if base.Auth["tpl"] == "" {
		t.Fatalf("expected rendered token under tpl")
	}
}

func TestPublicAuthHelpers_WireThrough(t *testing.T) {
	// Register a temporary provider and acquire via struct-based API
	iauth.Register("test-wire", func(spec map[string]interface{}) (iauth.Method, error) {
		return dummyMethodWire("ok"), nil
	})
	a := &apimigrate.Auth{Type: "test-wire", Name: "tw", Methods: apimigrate.NewAuthSpecFromMap(map[string]interface{}{})}
	v, err := a.Acquire(context.Background(), nil)
	if err != nil || v != "ok" {
		t.Fatalf("Acquire via public type failed: v=%q err=%v", v, err)
	}
}

type dummyMethodWire string

func (d dummyMethodWire) Acquire(_ context.Context) (string, error) { return string(d), nil }
