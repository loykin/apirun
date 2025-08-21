package auth

import (
	"context"
	"errors"
	"strings"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/clientcredentials"
)

// OAuth2Config holds configuration for generic OAuth2/OIDC token acquisition.
type OAuth2Config struct {
	Name        string   `mapstructure:"name"`
	Header      string   `mapstructure:"header"`
	ClientID    string   `mapstructure:"client_id"`
	ClientSec   string   `mapstructure:"client_secret"`
	RedirectURL string   `mapstructure:"redirect_url"`
	Scopes      []string `mapstructure:"scopes"`
	BaseURL     string   `mapstructure:"base_url"`
	AuthURL     string   `mapstructure:"auth_url"`
	TokenURL    string   `mapstructure:"token_url"`
	Username    string   `mapstructure:"username"`
	Password    string   `mapstructure:"password"`
}

// acquireOAuth2 obtains a Bearer token using golang.org/x/oauth2.
// It supports:
// - Resource Owner Password Credentials grant when Username and Password are provided.
// - Client Credentials grant when ClientID and ClientSec (secret) are provided.
// Note: token_url must be provided via config; it is no longer auto-derived from base_url/realm.
func acquireOAuth2(ctx context.Context, pc OAuth2Config) (string, string, error) {
	// Determine token URL: must be explicitly provided in config
	tokenURL := strings.TrimSpace(pc.TokenURL)
	if tokenURL == "" {
		return "", "", errors.New("oauth2: token_url is required")
	}
	clientID := strings.TrimSpace(pc.ClientID)
	clientSecret := strings.TrimSpace(pc.ClientSec)
	username := strings.TrimSpace(pc.Username)
	password := strings.TrimSpace(pc.Password)

	var tok *oauth2.Token
	var err error

	// Prefer password grant if username/password provided
	if username != "" && password != "" && clientID != "" {
		// Build oauth2.Config; AuthURL must be provided explicitly for password grant (no auto-derivation).
		authURL := strings.TrimSpace(pc.AuthURL)
		if authURL == "" {
			return "", "", errors.New("oauth2: auth_url is required for password grant")
		}
		ocfg := &oauth2.Config{
			ClientID:     clientID,
			ClientSecret: clientSecret,
			Endpoint: oauth2.Endpoint{
				AuthURL:  authURL,
				TokenURL: tokenURL,
			},
			Scopes: pc.Scopes,
		}
		tok, err = ocfg.PasswordCredentialsToken(ctx, username, password)
		if err != nil {
			return "", "", err
		}
	} else if clientID != "" && clientSecret != "" {
		cc := &clientcredentials.Config{
			ClientID:     clientID,
			ClientSecret: clientSecret,
			TokenURL:     tokenURL,
			Scopes:       pc.Scopes,
		}
		tok, err = cc.Token(ctx)
		if err != nil {
			return "", "", err
		}
	} else {
		return "", "", errors.New("oauth2: provide username/password with client_id, or client_id/client_secret")
	}

	if !tok.Valid() || strings.TrimSpace(tok.AccessToken) == "" {
		return "", "", errors.New("oauth2: received invalid token")
	}
	// Normalize header value; if TokenType is empty use Bearer.
	typ := tok.TokenType
	if strings.TrimSpace(typ) == "" {
		typ = "Bearer"
	}
	return headerOrDefault(pc.Header), strings.TrimSpace(typ) + " " + tok.AccessToken, nil
}
