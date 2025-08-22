package oauth2

import (
	"errors"
	"strings"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/clientcredentials"
)

// ClientCredentialsConfig holds configuration for the Client Credentials grant.
type ClientCredentialsConfig struct {
	Name      string   `mapstructure:"name"`
	Header    string   `mapstructure:"header"`
	ClientID  string   `mapstructure:"client_id"`
	ClientSec string   `mapstructure:"client_secret"`
	TokenURL  string   `mapstructure:"token_url"`
	Scopes    []string `mapstructure:"scopes"`
}

type clientCredentialsMethod struct{ c ClientCredentialsConfig }

func (m clientCredentialsMethod) Name() string { return m.c.Name }
func (m clientCredentialsMethod) Acquire(ctx Context) (string, string, error) {
	return acquireClientCredentials(ctx, m.c)
}

func acquireClientCredentials(ctx Context, c ClientCredentialsConfig) (string, string, error) {
	clientID := strings.TrimSpace(c.ClientID)
	clientSecret := strings.TrimSpace(c.ClientSec)
	tokenURL := strings.TrimSpace(c.TokenURL)
	if tokenURL == "" {
		return "", "", errors.New("oauth2: token_url is required for client_credentials grant")
	}
	if clientID == "" || clientSecret == "" {
		return "", "", errors.New("oauth2: client_id and client_secret are required for client_credentials grant")
	}
	cc := &clientcredentials.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		TokenURL:     tokenURL,
		Scopes:       c.Scopes,
		AuthStyle:    oauth2.AuthStyleInParams,
	}
	tok, err := cc.Token(ctx)
	if err != nil {
		return "", "", err
	}
	return normalizeOAuth2Token(c.Header, tok)
}
