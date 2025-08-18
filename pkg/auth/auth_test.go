package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAcquireToken_Keycloak_Success_DefaultHeaderBearer(t *testing.T) {
	// Mock Keycloak token endpoint
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(405)
			return
		}
		if ct := r.Header.Get("Content-Type"); !strings.Contains(ct, "application/x-www-form-urlencoded") {
			w.WriteHeader(415)
			return
		}
		// Respond with access_token
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{"access_token":"abc-123"}`))
	}))
	defer srv.Close()

	cfg := Config{Provider: ProviderConfig{
		Name:        "keycloak",
		BaseURL:     srv.URL,
		Realm:       "test",
		ClientID:    "client",
		Username:    "alice",
		Password:    "secret",
		TokenHeader: "", // default Authorization
	}}

	h, token, err := AcquireToken(context.Background(), cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.EqualFold(h, "authorization") {
		t.Fatalf("expected Authorization header, got %s", h)
	}
	if token != "Bearer abc-123" {
		t.Fatalf("expected Bearer prefix, got %q", token)
	}
}

func TestAcquireToken_Keycloak_CustomHeader_NoBearer(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{"access_token":"tkn"}`))
	}))
	defer srv.Close()

	cfg := Config{Provider: ProviderConfig{
		Name:        "keycloak",
		BaseURL:     srv.URL,
		Realm:       "test",
		ClientID:    "client",
		Username:    "alice",
		Password:    "secret",
		TokenHeader: "X-Token",
	}}

	h, token, err := AcquireToken(context.Background(), cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if h != "X-Token" {
		t.Fatalf("expected custom header, got %s", h)
	}
	if token != "tkn" {
		t.Fatalf("expected raw token without Bearer, got %q", token)
	}
}

func TestAcquireToken_PocketBase_UserAndAdmin(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		// Validate endpoint path
		if calls == 1 && r.URL.Path != "/api/users/auth-with-password" {
			w.WriteHeader(404)
			return
		}
		if calls == 2 && r.URL.Path != "/api/admins/auth-with-password" {
			w.WriteHeader(404)
			return
		}
		// Check body JSON has identity/password
		var body struct{ Identity, Password string }
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body.Identity == "" || body.Password == "" {
			w.WriteHeader(400)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		if calls == 1 {
			_, _ = w.Write([]byte(`{"token":"user-token"}`))
		} else {
			_, _ = w.Write([]byte(`{"token":"admin-token"}`))
		}
	}))
	defer srv.Close()

	// User login
	cfgUser := Config{Provider: ProviderConfig{
		Name:        "pocketbase",
		BaseURL:     srv.URL,
		Identity:    "user@example.com",
		Password:    "pw",
		IsAdmin:     false,
		TokenHeader: "",
	}}
	h1, tok1, err := AcquireToken(context.Background(), cfgUser)
	if err != nil {
		t.Fatalf("user login error: %v", err)
	}
	if !strings.EqualFold(h1, "authorization") || tok1 != "Bearer user-token" {
		t.Fatalf("unexpected user header/token: %s %q", h1, tok1)
	}

	// Admin login
	cfgAdmin := cfgUser
	cfgAdmin.Provider.IsAdmin = true
	h2, tok2, err := AcquireToken(context.Background(), cfgAdmin)
	if err != nil {
		t.Fatalf("admin login error: %v", err)
	}
	if !strings.EqualFold(h2, "authorization") || tok2 != "Bearer admin-token" {
		t.Fatalf("unexpected admin header/token: %s %q", h2, tok2)
	}
}

func TestAcquireToken_Goth_NotRegistered(t *testing.T) {
	cfg := Config{Provider: ProviderConfig{
		Name: "github", // not registered with goth in this test
	}}
	h, token, err := AcquireToken(context.Background(), cfg)
	if err == nil {
		t.Fatalf("expected error for unregistered goth provider")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "not registered") {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.EqualFold(h, "authorization") {
		t.Fatalf("expected default Authorization header, got %s", h)
	}
	if token != "" {
		t.Fatalf("expected empty token, got %q", token)
	}
}
