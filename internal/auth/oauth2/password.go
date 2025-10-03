package oauth2

import (
	"context"
	"fmt"
	"net/http"

	acommon "github.com/loykin/apirun/internal/auth/common"
	"github.com/loykin/apirun/internal/util"
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
	fields := util.TrimSpaceFields(m.c.ClientID, m.c.Username, m.c.Password, m.c.AuthURL, m.c.TokenURL)
	clientID, username, password, authURL, tokenURL := fields[0], fields[1], fields[2], fields[3], fields[4]
	if tokenURL == "" {
		return "", fmt.Errorf("oauth2: token_url is required for password grant")
	}
	if clientID == "" || username == "" || password == "" {
		return "", fmt.Errorf("oauth2: client_id, username and password are required for password grant")
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
	ocfg := &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: util.TrimWithDefault(m.c.ClientSec, ""),
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
