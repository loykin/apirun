package oauth2

import (
	"fmt"
	"strings"

	golangoauth2 "golang.org/x/oauth2"
)

// normalizeOAuth2Token builds the Authorization header value from an oauth2.Token.
func normalizeOAuth2Token(tok *golangoauth2.Token) (string, error) {
	if tok == nil || !tok.Valid() || strings.TrimSpace(tok.AccessToken) == "" {
		return "", fmt.Errorf("oauth2: received invalid token")
	}
	return tok.AccessToken, nil
}

// urlQueryEscape is a tiny helper to avoid importing net/url for simple cases.
func urlQueryEscape(s string) string {
	return strings.ReplaceAll(strings.ReplaceAll(s, " ", "+"), "\n", "%0A")
}
