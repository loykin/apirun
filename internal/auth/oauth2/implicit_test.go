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
