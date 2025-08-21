package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// Test OAuth2 password grant using golang.org/x/oauth2
func TestOAuth2_PasswordGrant(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", r.Method)
		}
		if ct := r.Header.Get("Content-Type"); !strings.Contains(ct, "application/x-www-form-urlencoded") {
			t.Fatalf("expected form content-type, got %s", ct)
		}
		if err := r.ParseForm(); err != nil {
			t.Fatalf("parse form: %v", err)
		}
		if got := r.PostForm.Get("grant_type"); got != "password" {
			t.Fatalf("expected grant_type=password, got %s", got)
		}
		if r.PostForm.Get("username") == "" || r.PostForm.Get("password") == "" {
			t.Fatalf("username/password missing in form: %v", r.PostForm)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"abc123","token_type":"Bearer","expires_in":300}`))
	}))
	defer srv.Close()

	spec := map[string]interface{}{
		"name":      "keycloak",
		"client_id": "admin-cli",
		"username":  "admin",
		"password":  "root",
		"token_url": srv.URL + "/realms/master/protocol/openid-connect/token",
		"auth_url":  srv.URL + "/realms/master/protocol/openid-connect/auth",
	}

	h, v, _, err := AcquireFromMap(context.Background(), "oauth2", spec)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.EqualFold(h, "authorization") {
		t.Fatalf("expected Authorization header, got %s", h)
	}
	if v != "Bearer abc123" {
		t.Fatalf("expected 'Bearer abc123', got %q", v)
	}
}

// Test OAuth2 client credentials grant
func TestOAuth2_ClientCredentials(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", r.Method)
		}
		if err := r.ParseForm(); err != nil {
			t.Fatalf("parse form: %v", err)
		}
		if r.PostForm.Get("grant_type") != "client_credentials" {
			t.Fatalf("expected client_credentials grant, got %s", r.PostForm.Get("grant_type"))
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"cc_token","token_type":"","expires_in":300}`))
	}))
	defer srv.Close()

	spec := map[string]interface{}{
		"name":          "svc-oauth2",
		"client_id":     "svc",
		"client_secret": "s3cr3t",
		"token_url":     srv.URL + "/realms/master/protocol/openid-connect/token",
	}

	h, v, _, err := AcquireFromMap(context.Background(), "oauth2", spec)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.ToLower(h) != "authorization" {
		t.Fatalf("expected Authorization header, got %s", h)
	}
	if v != "Bearer cc_token" { // TokenType was empty; code should default to Bearer
		t.Fatalf("expected 'Bearer cc_token', got %q", v)
	}
}

// Allow using explicit TokenURL without base/realm provided in config.
func TestOAuth2_ExplicitTokenURL_NoBaseRealm(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", r.Method)
		}
		if err := r.ParseForm(); err != nil {
			t.Fatalf("parse form: %v", err)
		}
		if gt := r.PostForm.Get("grant_type"); gt != "client_credentials" {
			t.Fatalf("expected client_credentials grant, got %s", gt)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"tok123","expires_in":3600}`))
	}))
	defer srv.Close()

	spec := map[string]interface{}{
		"name":          "custom-oauth2",
		"token_url":     srv.URL,
		"client_id":     "client",
		"client_secret": "secret",
	}

	h, v, _, err := AcquireFromMap(context.Background(), "oauth2", spec)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.ToLower(h) != "authorization" {
		t.Fatalf("expected Authorization header, got %s", h)
	}
	if v != "Bearer tok123" {
		t.Fatalf("expected 'Bearer tok123', got %q", v)
	}
}
