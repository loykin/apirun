package oauth2

import (
	"testing"
	"time"

	golangoauth2 "golang.org/x/oauth2"
)

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
