package oauth2

import (
	"context"
	"testing"
)

func TestAcquireImplicit_Success(t *testing.T) {
	cfg := ImplicitConfig{
		ClientID:    "webapp",
		RedirectURL: "http://localhost:3000/callback",
		AuthURL:     "http://auth.local/realms/demo/protocol/openid-connect/auth",
		Scopes:      []string{"openid", "profile"},
	}
	v, err := (implicitMethod{c: cfg}).Acquire(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v == "" || v[:4] != "http" {
		t.Fatalf("expected URL value, got %q", v)
	}
	if want := "response_type=token"; !contains(v, want) {
		t.Fatalf("expected %q in URL, got %q", want, v)
	}
}

func TestAcquireImplicit_ValidationErrors(t *testing.T) {
	_, err := (implicitMethod{c: ImplicitConfig{}}).Acquire(context.Background())
	if err == nil {
		t.Fatal("expected error for missing fields")
	}
}

// contains is a tiny helper to avoid importing strings
func contains(s, sub string) bool { return len(s) >= len(sub) && (indexOf(s, sub) >= 0) }

func indexOf(s, sub string) int {
	// naive search sufficient for tests
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

func TestInternalImplicitConfig_ToMap(t *testing.T) {
	c := ImplicitConfig{ClientID: "id", RedirectURL: "r", AuthURL: "a"}
	m := c.ToMap()
	if m["grant_type"] != "implicit" {
		t.Fatalf("grant_type mismatch: %+v", m)
	}
	sub := m["grant_config"].(map[string]interface{})
	if sub["client_id"] != "id" || sub["redirect_url"] != "r" || sub["auth_url"] != "a" {
		t.Fatalf("implicit grant_config mismatch: %+v", sub)
	}
	if _, ok := sub["scopes"]; ok {
		t.Fatalf("scopes should be absent when empty: %+v", sub)
	}
	c.Scopes = []string{"p"}
	sub2 := c.ToMap()["grant_config"].(map[string]interface{})
	if got, ok := sub2["scopes"].([]string); !ok || len(got) != 1 || got[0] != "p" {
		t.Fatalf("scopes not preserved: %+v", sub2["scopes"])
	}
}
