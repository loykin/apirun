package oauth2

import (
	"errors"
	"strings"
)

// ImplicitConfig holds configuration for the Implicit grant.
// Behavior: returns the header name and an authorization URL that includes response_type=token.
type ImplicitConfig struct {
	Name        string   `mapstructure:"name"`
	Header      string   `mapstructure:"header"`
	ClientID    string   `mapstructure:"client_id"`
	RedirectURL string   `mapstructure:"redirect_url"`
	AuthURL     string   `mapstructure:"auth_url"`
	Scopes      []string `mapstructure:"scopes"`
}

type implicitMethod struct{ c ImplicitConfig }

func (m implicitMethod) Name() string { return m.c.Name }
func (m implicitMethod) Acquire(_ Context) (string, string, error) {
	return acquireImplicit(m.c)
}

func acquireImplicit(c ImplicitConfig) (string, string, error) {
	clientID := strings.TrimSpace(c.ClientID)
	authURL := strings.TrimSpace(c.AuthURL)
	redirect := strings.TrimSpace(c.RedirectURL)
	if authURL == "" {
		return "", "", errors.New("oauth2: auth_url is required for implicit grant")
	}
	if clientID == "" {
		return "", "", errors.New("oauth2: client_id is required for implicit grant")
	}
	if redirect == "" {
		return "", "", errors.New("oauth2: redirect_url is required for implicit grant")
	}
	params := []string{
		"response_type=token",
		"client_id=" + urlQueryEscape(clientID),
		"redirect_uri=" + urlQueryEscape(redirect),
	}
	if len(c.Scopes) > 0 {
		params = append(params, "scope="+urlQueryEscape(strings.Join(c.Scopes, " ")))
	}
	u := strings.TrimRight(authURL, "?") + "?" + strings.Join(params, "&")
	return headerOrDefault(c.Header), u, nil
}
