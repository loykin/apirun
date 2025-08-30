package apimigrate

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	iauth "github.com/loykin/apimigrate/internal/auth"
)

// helper to decode application/x-www-form-urlencoded bodies in test servers
func parseForm(r *http.Request) url.Values {
	_ = r.ParseForm()
	return r.Form
}

func TestAcquireBasicAuth_Wrapper(t *testing.T) {
	ctx := context.Background()
	cfg := BasicAuthConfig{Username: "user", Password: "pass"}
	v, err := AcquireBasicAuthWithName(ctx, cfg)
	if err != nil {
		t.Fatalf("AcquireBasicAuthWithName error: %v", err)
	}
	// Now v is the bare base64 token without the "Basic " prefix
	if strings.HasPrefix(v, "Basic ") || v == "" {
		t.Fatalf("expected bare base64 token, got %q", v)
	}
}

func TestAcquireOAuth2Password_Wrapper(t *testing.T) {
	// token endpoint
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/token" {
			t.Fatalf("expected /token, got %s", r.URL.Path)
		}
		f := parseForm(r)
		if f.Get("grant_type") != "password" {
			t.Fatalf("grant_type expected password, got %s", f.Get("grant_type"))
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "t-pass", "token_type": "Bearer"})
	}))
	defer srv.Close()

	ctx := context.Background()
	cfg := OAuth2PasswordConfig{
		ClientID: "cid",
		TokenURL: srv.URL + "/token",
		Username: "u",
		Password: "p",
	}
	v, err := AcquireOAuth2PasswordWithName(ctx, cfg)
	if err != nil {
		t.Fatalf("AcquireOAuth2PasswordWithName error: %v", err)
	}
	if v != "t-pass" {
		t.Fatalf("unexpected token value: %q", v)
	}
}

func TestAcquireOAuth2ClientCredentials_Wrapper(t *testing.T) {
	// token endpoint
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/token" {
			t.Fatalf("expected /token, got %s", r.URL.Path)
		}
		f := parseForm(r)
		if f.Get("grant_type") != "client_credentials" {
			t.Fatalf("grant_type expected client_credentials, got %s", f.Get("grant_type"))
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "t-cc", "token_type": "Bearer"})
	}))
	defer srv.Close()

	ctx := context.Background()
	cfg := OAuth2ClientCredentialsConfig{
		ClientID:  "cid",
		ClientSec: "sec",
		TokenURL:  srv.URL + "/token",
	}
	v, err := AcquireOAuth2ClientCredentialsWithName(ctx, cfg)
	if err != nil {
		t.Fatalf("AcquireOAuth2ClientCredentialsWithName error: %v", err)
	}
	if v != "t-cc" {
		t.Fatalf("unexpected token value: %q", v)
	}
}

func TestAcquireOAuth2Implicit_Wrapper(t *testing.T) {
	ctx := context.Background()
	cfg := OAuth2ImplicitConfig{
		ClientID:    "cid",
		RedirectURL: "http://localhost/redirect",
		AuthURL:     "http://auth.example/authorize",
		Scopes:      []string{"read", "write"},
	}
	v, err := AcquireOAuth2ImplicitWithName(ctx, cfg)
	if err != nil {
		t.Fatalf("AcquireOAuth2ImplicitWithName error: %v", err)
	}
	// value is an URL containing response_type=token and params
	if !strings.Contains(v, "response_type=token") || !strings.Contains(v, "client_id=cid") {
		t.Fatalf("implicit url missing params: %q", v)
	}
	if !strings.Contains(v, "redirect_uri=") {
		t.Fatalf("implicit url missing redirect_uri: %q", v)
	}
	if !strings.Contains(v, "scope=") {
		t.Fatalf("implicit url missing scope: %q", v)
	}
}

func TestAcquirePocketBase_Wrapper(t *testing.T) {
	// mock pocketbase endpoint
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/admins/auth-with-password" {
			t.Fatalf("expected pocketbase login path, got %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"token": "pb-token"})
	}))
	defer srv.Close()

	ctx := context.Background()
	cfg := PocketBaseAuthConfig{
		BaseURL:  srv.URL,
		Email:    "a@b.c",
		Password: "secret",
	}
	v, err := AcquirePocketBaseWithName(ctx, cfg)
	if err != nil {
		t.Fatalf("AcquirePocketBaseWithName error: %v", err)
	}
	if v != "pb-token" {
		t.Fatalf("unexpected token value: %q", v)
	}
}

func TestRegistry_LoadsWithPublicConstants_Basic(t *testing.T) {
	ctx := context.Background()
	// Use registry directly with public constant
	v, err := iauth.AcquireAndStoreWithName(ctx, AuthTypeBasic, map[string]interface{}{
		"username": "user",
		"password": "pass",
	})
	if err != nil || v == "" {
		t.Fatalf("AcquireAndStoreWithName basic error: v=%q err=%v", v, err)
	}
}

func TestRegistry_LoadsWithPublicConstants_OAuth2Password(t *testing.T) {
	// mock token endpoint
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/token" {
			t.Fatalf("expected /token, got %s", r.URL.Path)
		}
		f := parseForm(r)
		if f.Get("grant_type") != "password" {
			t.Fatalf("grant_type expected password, got %s", f.Get("grant_type"))
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "t-pass2", "token_type": "Bearer"})
	}))
	defer srv.Close()

	ctx := context.Background()
	spec := map[string]interface{}{
		"grant_type": "password",
		"grant_config": map[string]interface{}{
			"client_id": "cid",
			"token_url": srv.URL + "/token",
			"username":  "u",
			"password":  "p",
		},
	}
	v, err := iauth.AcquireAndStoreWithName(ctx, AuthTypeOAuth2, spec)
	if err != nil || v != "t-pass2" {
		t.Fatalf("AcquireAndStoreWithName oauth2 password error: v=%q err=%v", v, err)
	}
}

func TestRegistry_LoadsWithPublicConstants_PocketBase(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/admins/auth-with-password" {
			t.Fatalf("expected pocketbase login path, got %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"token": "pb-token-2"})
	}))
	defer srv.Close()

	ctx := context.Background()
	spec := map[string]interface{}{
		"base_url": srv.URL,
		"email":    "a@b.c",
		"password": "secret",
	}
	v, err := iauth.AcquireAndStoreWithName(ctx, AuthTypePocketBase, spec)
	if err != nil || v != "pb-token-2" {
		t.Fatalf("AcquireAndStoreWithName pocketbase error: v=%q err=%v", v, err)
	}
}
