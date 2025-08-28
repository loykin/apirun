package oauth2

import (
	"context"
	"errors"
	"strings"

	"golang.org/x/oauth2"
)

// PasswordConfig holds configuration for the Resource Owner Password Credentials grant.
type PasswordConfig struct {
	Name      string   `mapstructure:"name"`
	Header    string   `mapstructure:"header"`
	ClientID  string   `mapstructure:"client_id"`
	ClientSec string   `mapstructure:"client_secret"`
	AuthURL   string   `mapstructure:"auth_url"`
	TokenURL  string   `mapstructure:"token_url"`
	Username  string   `mapstructure:"username"`
	Password  string   `mapstructure:"password"`
	Scopes    []string `mapstructure:"scopes"`
}

type passwordMethod struct {
	c PasswordConfig
}

func (m passwordMethod) Name() string {
	return m.c.Name
}

func (m passwordMethod) Acquire(ctx context.Context) (string, string, error) {
	clientID := strings.TrimSpace(m.c.ClientID)
	username := strings.TrimSpace(m.c.Username)
	password := strings.TrimSpace(m.c.Password)
	authURL := strings.TrimSpace(m.c.AuthURL)
	tokenURL := strings.TrimSpace(m.c.TokenURL)
	if tokenURL == "" {
		return "", "", errors.New("oauth2: token_url is required for password grant")
	}
	if clientID == "" || username == "" || password == "" {
		return "", "", errors.New("oauth2: client_id, username and password are required for password grant")
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
		return "", "", err
	}
	return normalizeOAuth2Token(m.c.Header, tok)
}
