package oauth2

import (
	"errors"
	"strings"

	"github.com/loykin/apimigrate/internal/auth/common"
	golangoauth2 "golang.org/x/oauth2"
)

// normalizeOAuth2Token builds the Authorization header value from an oauth2.Token.
func normalizeOAuth2Token(header string, tok *golangoauth2.Token) (string, string, error) {
	if tok == nil || !tok.Valid() || strings.TrimSpace(tok.AccessToken) == "" {
		return "", "", errors.New("oauth2: received invalid token")
	}
	typ := strings.TrimSpace(tok.TokenType)
	if typ == "" {
		typ = "Bearer"
	}
	return common.HeaderOrDefault(header), typ + " " + tok.AccessToken, nil
}

// urlQueryEscape is a tiny helper to avoid importing net/url for simple cases.
func urlQueryEscape(s string) string {
	return strings.ReplaceAll(strings.ReplaceAll(s, " ", "+"), "\n", "%0A")
}
