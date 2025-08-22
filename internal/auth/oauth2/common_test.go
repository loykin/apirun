package oauth2

import (
	"context"
	"testing"
	"time"

	golangoauth2 "golang.org/x/oauth2"
)

func TestHeaderOrDefault(t *testing.T) {
	if got := headerOrDefault(""); got != "Authorization" {
		t.Fatalf("expected Authorization, got %q", got)
	}
	if got := headerOrDefault("  X-Api-Key  "); got != "X-Api-Key" {
		t.Fatalf("expected X-Api-Key, got %q", got)
	}
}

func TestNormalizeOAuth2Token_Success(t *testing.T) {
	tok := &golangoauth2.Token{AccessToken: "abc123", TokenType: "Bearer", Expiry: time.Now().Add(time.Hour)}
	h, v, err := normalizeOAuth2Token("", tok)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if h != "Authorization" {
		t.Fatalf("unexpected header: %q", h)
	}
	if v != "Bearer abc123" {
		t.Fatalf("unexpected value: %q", v)
	}
}

func TestNormalizeOAuth2Token_Error(t *testing.T) {
	// nil token
	if _, _, err := normalizeOAuth2Token("Authorization", nil); err == nil {
		t.Fatal("expected error for nil token")
	}
	// empty token
	tok := &golangoauth2.Token{}
	if _, _, err := normalizeOAuth2Token("Authorization", tok); err == nil {
		t.Fatal("expected error for empty token")
	}
}

func TestURLQueryEscape(t *testing.T) {
	in := "a b\nc"
	out := urlQueryEscape(in)
	if out != "a+b%0Ac" {
		t.Fatalf("unexpected escape result: %q", out)
	}
}

// small helper to ensure Acquire signature compiles with context
func TestMethodAcquireSignature(t *testing.T) {
	// Use a dummy Method via adapter to ensure types line up
	d := Adapter{M: dummyMethod{name: "x"}}
	_, _, _ = d.Acquire(context.Background())
}

type dummyMethod struct{ name string }

func (d dummyMethod) Name() string { return d.name }
func (d dummyMethod) Acquire(_ Context) (string, string, error) {
	return "H", "V", nil
}
