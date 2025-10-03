package oauth2

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	acommon "github.com/loykin/apirun/internal/auth/common"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/clientcredentials"
)

// ClientCredentialsConfig holds configuration for the Client Credentials grant.
type ClientCredentialsConfig struct {
	ClientID  string   `mapstructure:"client_id"`
	ClientSec string   `mapstructure:"client_secret"`
	TokenURL  string   `mapstructure:"token_url"`
	Scopes    []string `mapstructure:"scopes"`
}

// ToMap returns a spec for oauth2 client_credentials
func (c ClientCredentialsConfig) ToMap() map[string]interface{} {
	sub := map[string]interface{}{
		"client_id":     c.ClientID,
		"client_secret": c.ClientSec,
		"token_url":     c.TokenURL,
	}
	if len(c.Scopes) > 0 {
		sub["scopes"] = c.Scopes
	}
	return map[string]interface{}{
		"grant_type":   "client_credentials",
		"grant_config": sub,
	}
}

type clientCredentialsMethod struct {
	c ClientCredentialsConfig
}

func (m clientCredentialsMethod) Acquire(ctx context.Context) (string, error) {
	clientID := strings.TrimSpace(m.c.ClientID)
	clientSecret := strings.TrimSpace(m.c.ClientSec)
	tokenURL := strings.TrimSpace(m.c.TokenURL)
	if tokenURL == "" {
		return "", fmt.Errorf("oauth2: token_url is required for client_credentials grant")
	}
	if clientID == "" || clientSecret == "" {
		return "", fmt.Errorf("oauth2: client_id and client_secret are required for client_credentials grant")
	}
	// If a TLS config is provided, inject a custom HTTP client into the context
	if cfg := acommon.GetTLSConfig(); cfg != nil {
		tr := &http.Transport{TLSClientConfig: cfg}
		hc := &http.Client{Transport: tr}
		if ctx == nil {
			ctx = context.Background()
		}
		ctx = context.WithValue(ctx, oauth2.HTTPClient, hc)
	}
	cc := &clientcredentials.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		TokenURL:     tokenURL,
		Scopes:       m.c.Scopes,
		AuthStyle:    oauth2.AuthStyleInParams,
	}
	tok, err := cc.Token(ctx)
	if err != nil {
		return "", err
	}
	return normalizeOAuth2Token(tok)
}
