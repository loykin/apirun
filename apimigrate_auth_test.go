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
	iauth.ClearTokens()
	ctx := context.Background()
	cfg := BasicAuthConfig{Name: "b1", Username: "user", Password: "pass"}
	h, v, name, err := AcquireBasicAuth(ctx, cfg)
	if err != nil {
		t.Fatalf("AcquireBasicAuth error: %v", err)
	}
	if name != "b1" {
		t.Fatalf("unexpected name: %q", name)
	}
	if h != "Authorization" {
		t.Fatalf("unexpected header: %q", h)
	}
	if !strings.HasPrefix(v, "Basic ") {
		t.Fatalf("expected Basic prefix, got %q", v)
	}
	// verify token stored
	sh, sv, ok := iauth.GetToken("b1")
	if !ok || sh != h || sv != v {
		t.Fatalf("stored token mismatch: ok=%v, h=%q v=%q", ok, sh, sv)
	}
}

func TestAcquireOAuth2Password_Wrapper(t *testing.T) {
	iauth.ClearTokens()
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
		Name:     "pw1",
		ClientID: "cid",
		TokenURL: srv.URL + "/token",
		Username: "u",
		Password: "p",
	}
	h, v, name, err := AcquireOAuth2Password(ctx, cfg)
	if err != nil {
		t.Fatalf("AcquireOAuth2Password error: %v", err)
	}
	if name != "pw1" {
		t.Fatalf("unexpected name: %q", name)
	}
	if h != "Authorization" || v != "Bearer t-pass" {
		t.Fatalf("unexpected header/value: %q %q", h, v)
	}
}

func TestAcquireOAuth2ClientCredentials_Wrapper(t *testing.T) {
	iauth.ClearTokens()
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
		Name:      "cc1",
		ClientID:  "cid",
		ClientSec: "sec",
		TokenURL:  srv.URL + "/token",
	}
	h, v, name, err := AcquireOAuth2ClientCredentials(ctx, cfg)
	if err != nil {
		t.Fatalf("AcquireOAuth2ClientCredentials error: %v", err)
	}
	if name != "cc1" {
		t.Fatalf("unexpected name: %q", name)
	}
	if h != "Authorization" || v != "Bearer t-cc" {
		t.Fatalf("unexpected header/value: %q %q", h, v)
	}
}

func TestAcquireOAuth2Implicit_Wrapper(t *testing.T) {
	iauth.ClearTokens()
	ctx := context.Background()
	cfg := OAuth2ImplicitConfig{
		Name:        "impl1",
		ClientID:    "cid",
		RedirectURL: "http://localhost/redirect",
		AuthURL:     "http://auth.example/authorize",
		Scopes:      []string{"read", "write"},
	}
	h, v, name, err := AcquireOAuth2Implicit(ctx, cfg)
	if err != nil {
		t.Fatalf("AcquireOAuth2Implicit error: %v", err)
	}
	if name != "impl1" {
		t.Fatalf("unexpected name: %q", name)
	}
	if h != "Authorization" {
		t.Fatalf("unexpected header: %q", h)
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
	iauth.ClearTokens()
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
		Name:     "pb1",
		BaseURL:  srv.URL,
		Email:    "a@b.c",
		Password: "secret",
	}
	h, v, name, err := AcquirePocketBase(ctx, cfg)
	if err != nil {
		t.Fatalf("AcquirePocketBase error: %v", err)
	}
	if name != "pb1" {
		t.Fatalf("unexpected name: %q", name)
	}
	if h != "Authorization" || v != "pb-token" {
		t.Fatalf("unexpected header/value: %q %q", h, v)
	}
}
