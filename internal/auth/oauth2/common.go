package oauth2

import (
	"context"
	"errors"
	"strings"

	golangoauth2 "golang.org/x/oauth2"
)

// Method is the local interface implemented by oauth2 auth methods.
// It matches the parent auth.Method signature (Name, Acquire).
type Method interface {
	Name() string
	Acquire(ctx Context) (header string, value string, err error)
}

// Context is an alias to standard context.Context to avoid repeating imports.
type Context = context.Context

// headerOrDefault returns Authorization if empty
func headerOrDefault(h string) string {
	h = strings.TrimSpace(h)
	if h == "" {
		return "Authorization"
	}
	return h
}

// normalizeOAuth2Token builds the Authorization header value from an oauth2.Token.
func normalizeOAuth2Token(header string, tok *golangoauth2.Token) (string, string, error) {
	if tok == nil || !tok.Valid() || strings.TrimSpace(tok.AccessToken) == "" {
		return "", "", errors.New("oauth2: received invalid token")
	}
	typ := strings.TrimSpace(tok.TokenType)
	if typ == "" {
		typ = "Bearer"
	}
	return headerOrDefault(header), typ + " " + tok.AccessToken, nil
}

// urlQueryEscape is a tiny helper to avoid importing net/url for simple cases.
func urlQueryEscape(s string) string {
	return strings.ReplaceAll(strings.ReplaceAll(s, " ", "+"), "\n", "%0A")
}
