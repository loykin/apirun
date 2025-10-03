package oauth2

import (
	"context"
	"fmt"
	"strings"
)

// ImplicitConfig holds configuration for the Implicit grant.
// Behavior: returns the header name and an authorization URL that includes response_type=token.
type ImplicitConfig struct {
	ClientID    string   `mapstructure:"client_id"`
	RedirectURL string   `mapstructure:"redirect_url"`
	AuthURL     string   `mapstructure:"auth_url"`
	Scopes      []string `mapstructure:"scopes"`
}

// ToMap returns a spec for oauth2 implicit grant
func (c ImplicitConfig) ToMap() map[string]interface{} {
	sub := map[string]interface{}{
		"client_id":    c.ClientID,
		"redirect_url": c.RedirectURL,
		"auth_url":     c.AuthURL,
	}
	if len(c.Scopes) > 0 {
		sub["scopes"] = c.Scopes
	}
	return map[string]interface{}{
		"grant_type":   "implicit",
		"grant_config": sub,
	}
}

type implicitMethod struct {
	c ImplicitConfig
}

func (m implicitMethod) Acquire(_ context.Context) (string, error) {
	clientID := strings.TrimSpace(m.c.ClientID)
	authURL := strings.TrimSpace(m.c.AuthURL)
	redirect := strings.TrimSpace(m.c.RedirectURL)
	if authURL == "" {
		return "", fmt.Errorf("oauth2: auth_url is required for implicit grant")
	}
	if clientID == "" {
		return "", fmt.Errorf("oauth2: client_id is required for implicit grant")
	}
	if redirect == "" {
		return "", fmt.Errorf("oauth2: redirect_url is required for implicit grant")
	}
	params := []string{
		"response_type=token",
		"client_id=" + urlQueryEscape(clientID),
		"redirect_uri=" + urlQueryEscape(redirect),
	}
	if len(m.c.Scopes) > 0 {
		params = append(params, "scope="+urlQueryEscape(strings.Join(m.c.Scopes, " ")))
	}
	u := strings.TrimRight(authURL, "?") + "?" + strings.Join(params, "&")
	return u, nil
}
