package oauth2

import (
	"context"
	"errors"
	"strings"

	"golang.org/x/oauth2"
)

// PasswordConfig holds configuration for the Resource Owner Password Credentials grant.
type PasswordConfig struct {
	ClientID  string   `mapstructure:"client_id"`
	ClientSec string   `mapstructure:"client_secret"`
	AuthURL   string   `mapstructure:"auth_url"`
	TokenURL  string   `mapstructure:"token_url"`
	Username  string   `mapstructure:"username"`
	Password  string   `mapstructure:"password"`
	Scopes    []string `mapstructure:"scopes"`
}

// ToMap returns a spec compatible with the oauth2 provider factory (password grant).
func (c PasswordConfig) ToMap() map[string]interface{} {
	sub := map[string]interface{}{
		"client_id":     c.ClientID,
		"client_secret": c.ClientSec,
		"auth_url":      c.AuthURL,
		"token_url":     c.TokenURL,
		"username":      c.Username,
		"password":      c.Password,
	}
	if len(c.Scopes) > 0 {
		sub["scopes"] = c.Scopes
	}
	return map[string]interface{}{
		"grant_type":   "password",
		"grant_config": sub,
	}
}

type passwordMethod struct {
	c PasswordConfig
}

func (m passwordMethod) Acquire(ctx context.Context) (string, error) {
	clientID := strings.TrimSpace(m.c.ClientID)
	username := strings.TrimSpace(m.c.Username)
	password := strings.TrimSpace(m.c.Password)
	authURL := strings.TrimSpace(m.c.AuthURL)
	tokenURL := strings.TrimSpace(m.c.TokenURL)
	if tokenURL == "" {
		return "", errors.New("oauth2: token_url is required for password grant")
	}
	if clientID == "" || username == "" || password == "" {
		return "", errors.New("oauth2: client_id, username and password are required for password grant")
	}
	ocfg := &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: strings.TrimSpace(m.c.ClientSec),
		Endpoint: oauth2.Endpoint{
			AuthURL:   authURL,
			TokenURL:  tokenURL,
			AuthStyle: oauth2.AuthStyleInParams,
		},
		Scopes: m.c.Scopes,
	}
	tok, err := ocfg.PasswordCredentialsToken(ctx, username, password)
	if err != nil {
		return "", err
	}
	return normalizeOAuth2Token(tok)
}
