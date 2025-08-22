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
		Name:     "pw",
		Header:   "",
		ClientID: "client",
		AuthURL:  srv.URL + "/auth",
		TokenURL: srv.URL + "/token",
		Username: "user",
		Password: "pass",
	}
	h, v, err := acquirePassword(context.Background(), cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if h != "Authorization" {
		t.Fatalf("unexpected header: %q", h)
	}
	if v != "Bearer t-pass" {
		t.Fatalf("unexpected value: %q", v)
	}
}

func TestAcquirePassword_ValidationErrors(t *testing.T) {
	_, _, err := acquirePassword(context.Background(), PasswordConfig{})
	if err == nil {
		t.Fatal("expected error for missing fields")
	}
}
