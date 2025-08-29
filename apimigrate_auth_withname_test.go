package apimigrate

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	iauth "github.com/loykin/apimigrate/internal/auth"
)

func TestAcquireAuthByProviderSpecWithName_Basic(t *testing.T) {
	iauth.ClearTokens()
	ctx := context.Background()
	// Spec without name; we will supply name via API
	spec := map[string]any{
		"username": "user",
		"password": "pass",
	}
	h, v, name, err := AcquireAuthByProviderSpecWithName(ctx, "basic", "explicit_basic", spec)
	if err != nil {
		t.Fatalf("AcquireAuthByProviderSpecWithName error: %v", err)
	}
	if name != "explicit_basic" {
		t.Fatalf("expected stored name 'explicit_basic', got %q", name)
	}
	sh, sv, ok := iauth.GetToken("explicit_basic")
	if !ok || sh != h || sv != v {
		t.Fatalf("token not stored under explicit name: ok=%v h=%q v=%q", ok, sh, sv)
	}
}

func TestAcquireBasicAuthWithName_IgnoresCfgName(t *testing.T) {
	iauth.ClearTokens()
	ctx := context.Background()
	cfg := BasicAuthConfig{Name: "ignored_name", Username: "u", Password: "p"}
	h, v, name, err := AcquireBasicAuthWithName(ctx, "basic_named", cfg)
	if err != nil {
		t.Fatalf("AcquireBasicAuthWithName error: %v", err)
	}
	if name != "basic_named" {
		t.Fatalf("expected stored name 'basic_named', got %q", name)
	}
	// ensure stored under provided name
	sh, sv, ok := iauth.GetToken("basic_named")
	if !ok || sh != h || sv != v {
		t.Fatalf("stored token mismatch: ok=%v h=%q v=%q", ok, sh, sv)
	}
}

func TestAcquireOAuth2ClientCredentialsWithName_IgnoresCfgName(t *testing.T) {
	iauth.ClearTokens()
	// mock token endpoint for client_credentials
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/token" {
			t.Fatalf("expected /token, got %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "t-cc2", "token_type": "Bearer"})
	}))
	defer srv.Close()

	ctx := context.Background()
	cfg := OAuth2ClientCredentialsConfig{
		Name:      "ignored_cc",
		ClientID:  "cid",
		ClientSec: "sec",
		TokenURL:  srv.URL + "/token",
	}
	h, v, name, err := AcquireOAuth2ClientCredentialsWithName(ctx, "cc_named", cfg)
	if err != nil {
		t.Fatalf("AcquireOAuth2ClientCredentialsWithName error: %v", err)
	}
	if name != "cc_named" {
		t.Fatalf("expected stored name 'cc_named', got %q", name)
	}
	// stored token check
	sh, sv, ok := iauth.GetToken("cc_named")
	if !ok || sh != h || sv != v {
		t.Fatalf("stored token mismatch: ok=%v h=%q v=%q", ok, sh, sv)
	}
}

func TestAcquireOAuth2PasswordWithName_IgnoresCfgName(t *testing.T) {
	iauth.ClearTokens()
	// mock token endpoint for password
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/token" {
			t.Fatalf("expected /token, got %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "t-pass2", "token_type": "Bearer"})
	}))
	defer srv.Close()

	ctx := context.Background()
	cfg := OAuth2PasswordConfig{
		Name:     "ignored_pw",
		ClientID: "cid",
		TokenURL: srv.URL + "/token",
		Username: "u",
		Password: "p",
	}
	h, v, name, err := AcquireOAuth2PasswordWithName(ctx, "pw_named", cfg)
	if err != nil {
		t.Fatalf("AcquireOAuth2PasswordWithName error: %v", err)
	}
	if name != "pw_named" {
		t.Fatalf("expected stored name 'pw_named', got %q", name)
	}
	sh, sv, ok := iauth.GetToken("pw_named")
	if !ok || sh != h || sv != v {
		t.Fatalf("stored token mismatch: ok=%v h=%q v=%q", ok, sh, sv)
	}
}

func TestAcquireOAuth2ImplicitWithName_IgnoresCfgName(t *testing.T) {
	iauth.ClearTokens()
	ctx := context.Background()
	cfg := OAuth2ImplicitConfig{
		Name:        "ignored_impl",
		ClientID:    "cid",
		RedirectURL: "http://localhost/redirect",
		AuthURL:     "http://auth.example/authorize",
	}
	h, v, name, err := AcquireOAuth2ImplicitWithName(ctx, "impl_named", cfg)
	if err != nil {
		t.Fatalf("AcquireOAuth2ImplicitWithName error: %v", err)
	}
	if name != "impl_named" {
		t.Fatalf("expected stored name 'impl_named', got %q", name)
	}
	if h == "" || v == "" {
		t.Fatalf("expected non-empty header/value for implicit")
	}
}

func TestAcquirePocketBaseWithName_IgnoresCfgName(t *testing.T) {
	iauth.ClearTokens()
	// mock pocketbase endpoint
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/admins/auth-with-password" {
			t.Fatalf("expected pocketbase login path, got %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"token": "pb-token2"})
	}))
	defer srv.Close()

	ctx := context.Background()
	cfg := PocketBaseAuthConfig{
		Name:     "ignored_pb",
		BaseURL:  srv.URL,
		Email:    "a@b.c",
		Password: "secret",
	}
	h, v, name, err := AcquirePocketBaseWithName(ctx, "pb_named", cfg)
	if err != nil {
		t.Fatalf("AcquirePocketBaseWithName error: %v", err)
	}
	if name != "pb_named" {
		t.Fatalf("expected stored name 'pb_named', got %q", name)
	}
	sh, sv, ok := iauth.GetToken("pb_named")
	if !ok || sh != h || sv != v {
		t.Fatalf("stored token mismatch: ok=%v h=%q v=%q", ok, sh, sv)
	}
}
