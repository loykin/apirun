package oauth2

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAcquireClientCredentials_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(tokenResp{AccessToken: "t-cc", TokenType: "Bearer"})
	}))
	defer srv.Close()

	cfg := ClientCredentialsConfig{
		ClientID:  "svc",
		ClientSec: "secret",
		TokenURL:  srv.URL + "/token",
	}
	v, err := (clientCredentialsMethod{c: cfg}).Acquire(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v != "t-cc" {
		t.Fatalf("unexpected value: %q", v)
	}
}

func TestAcquireClientCredentials_ValidationErrors(t *testing.T) {
	_, err := (clientCredentialsMethod{c: ClientCredentialsConfig{}}).Acquire(context.Background())
	if err == nil {
		t.Fatal("expected error for missing fields")
	}
}

func TestInternalClientCredentialsConfig_ToMap(t *testing.T) {
	c := ClientCredentialsConfig{ClientID: "id", ClientSec: "sec", TokenURL: "tok"}
	m := c.ToMap()
	if m["grant_type"] != "client_credentials" {
		t.Fatalf("grant_type mismatch: %+v", m)
	}
	sub := m["grant_config"].(map[string]interface{})
	if sub["client_id"] != "id" || sub["client_secret"] != "sec" || sub["token_url"] != "tok" {
		t.Fatalf("cc grant_config mismatch: %+v", sub)
	}
	if _, ok := sub["scopes"]; ok {
		t.Fatalf("scopes should be absent when empty: %+v", sub)
	}
	c.Scopes = []string{"s1", "s2"}
	sub2 := c.ToMap()["grant_config"].(map[string]interface{})
	if got, ok := sub2["scopes"].([]string); !ok || len(got) != 2 || got[0] != "s1" || got[1] != "s2" {
		t.Fatalf("scopes not preserved: %+v", sub2["scopes"])
	}
}
