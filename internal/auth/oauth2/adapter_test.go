package oauth2

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGetGrantMethod_Password_WithGrantConfig(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(tokenResp{AccessToken: "t1", TokenType: "Bearer"})
	}))
	defer srv.Close()

	c := Auth2Config{
		GrantType: "password",
		GrantConfig: map[string]interface{}{
			"client_id": "c",
			"username":  "u",
			"password":  "p",
			"auth_url":  srv.URL + "/auth",
			"token_url": srv.URL + "/token",
		},
	}
	m, err := c.GetGrantMethod()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	h, v, err := m.Acquire(context.Background())
	if err != nil {
		t.Fatalf("unexpected acquire error: %v", err)
	}
	if h != "Authorization" || v != "Bearer t1" {
		t.Fatalf("unexpected token: %s %s", h, v)
	}
}

func TestGetGrantMethod_ClientCredentials_WithGrantConfig(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(tokenResp{AccessToken: "t2", TokenType: "Bearer"})
	}))
	defer srv.Close()

	c := Auth2Config{
		GrantType: "client_credentials",
		GrantConfig: map[string]interface{}{
			"client_id":     "c",
			"client_secret": "s",
			"token_url":     srv.URL + "/token",
		},
	}
	m, err := c.GetGrantMethod()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	h, v, err := m.Acquire(context.Background())
	if err != nil {
		t.Fatalf("unexpected acquire error: %v", err)
	}
	if h != "Authorization" || v != "Bearer t2" {
		t.Fatalf("unexpected token: %s %s", h, v)
	}
}

func TestGetGrantMethod_Implicit_WithGrantConfig(t *testing.T) {
	c := Auth2Config{
		GrantType: "implicit",
		GrantConfig: map[string]interface{}{
			"client_id":    "web",
			"redirect_url": "http://localhost/cb",
			"auth_url":     "http://auth/authorize",
		},
	}
	m, err := c.GetGrantMethod()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	h, v, err := m.Acquire(context.Background())
	if err != nil {
		t.Fatalf("unexpected acquire error: %v", err)
	}
	if h != "Authorization" {
		t.Fatalf("unexpected header: %s", h)
	}
	if v == "" || v[:4] != "http" {
		t.Fatalf("expected URL, got %q", v)
	}
}

func TestGetGrantMethod_MissingGrantType_Error(t *testing.T) {
	_, err := (Auth2Config{}).GetGrantMethod()
	if err == nil {
		t.Fatal("expected error for missing grant_type")
	}
}
