package oauth2

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

type tokenResp struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
}

func TestAcquirePassword_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		// minimal: just return a token regardless of params
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(tokenResp{AccessToken: "t-pass", TokenType: "Bearer"})
	}))
	defer srv.Close()

	cfg := PasswordConfig{
		ClientID: "client",
		AuthURL:  srv.URL + "/auth",
		TokenURL: srv.URL + "/token",
		Username: "user",
		Password: "pass",
	}
	v, err := (passwordMethod{c: cfg}).Acquire(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v != "t-pass" {
		t.Fatalf("unexpected value: %q", v)
	}
}

func TestAcquirePassword_ValidationErrors(t *testing.T) {
	_, err := (passwordMethod{c: PasswordConfig{}}).Acquire(context.Background())
	if err == nil {
		t.Fatal("expected error for missing fields")
	}
}

func TestInternalPasswordConfig_ToMap(t *testing.T) {
	c := PasswordConfig{
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
	sub := m["grant_config"].(map[string]interface{})
	if sub["client_id"] != "cid" || sub["client_secret"] != "sec" || sub["auth_url"] != "a" || sub["token_url"] != "t" || sub["username"] != "u" || sub["password"] != "p" {
		t.Fatalf("password grant_config mismatch: %+v", sub)
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
