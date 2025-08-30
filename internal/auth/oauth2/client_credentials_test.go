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
