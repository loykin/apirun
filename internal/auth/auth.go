package auth

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/go-resty/resty/v2"
	"github.com/markbates/goth"
)

type Config struct {
	Provider ProviderConfig `yaml:"provider" json:"provider" mapstructure:"provider"`
}

type ProviderConfig struct {
	Name   string `yaml:"name" json:"name" mapstructure:"name"`
	Header string `yaml:"header" json:"header" mapstructure:"header"`
	// For Keycloak
	TokenURL  string `yaml:"token_url" json:"token_url" mapstructure:"token_url"`
	ClientID  string `yaml:"client_id" json:"client_id" mapstructure:"client_id"`
	ClientSec string `yaml:"client_secret" json:"client_secret" mapstructure:"client_secret"`
	Username  string `yaml:"username" json:"username" mapstructure:"username"`
	Password  string `yaml:"password" json:"password" mapstructure:"password"`
	// For PocketBase
	BaseURL string `yaml:"base_url" json:"base_url" mapstructure:"base_url"`
	Email   string `yaml:"email" json:"email" mapstructure:"email"`
}

func AcquireToken(ctx context.Context, cfg Config) (string, string, error) {
	pc := cfg.Provider
	if strings.TrimSpace(pc.Name) == "" {
		return "", "", errors.New("provider name is required")
	}
	switch strings.ToLower(pc.Name) {
	case "keycloak":
		return acquireKeycloak(ctx, pc)
	case "pocketbase":
		return acquirePocketBase(ctx, pc)
	default:
		// Try goth (e.g., github)
		return AcquireTokenWithGoth(ctx, pc)
	}
}

func AcquireTokenWithGoth(_ context.Context, pc ProviderConfig) (string, string, error) {
	provider, err := goth.GetProvider(pc.Name)
	if err != nil {
		return "", "", fmt.Errorf("goth provider %s not found: %w", pc.Name, err)
	}
	sess, err := provider.BeginAuth("state")
	if err != nil {
		return "", "", err
	}
	authURL, err := sess.GetAuthURL()
	if err != nil {
		return "", "", err
	}
	return headerOrDefault(pc.Header), authURL, nil
}

func acquireKeycloak(ctx context.Context, pc ProviderConfig) (string, string, error) {
	if strings.TrimSpace(pc.TokenURL) == "" || strings.TrimSpace(pc.ClientID) == "" || strings.TrimSpace(pc.Username) == "" || strings.TrimSpace(pc.Password) == "" {
		return "", "", errors.New("keycloak: token_url, client_id, username and password are required")
	}
	form := url.Values{}
	form.Set("grant_type", "password")
	form.Set("client_id", pc.ClientID)
	if strings.TrimSpace(pc.ClientSec) != "" {
		form.Set("client_secret", pc.ClientSec)
	}
	form.Set("username", pc.Username)
	form.Set("password", pc.Password)

	resp, err := resty.New().R().SetContext(ctx).SetHeader("Content-Type", "application/x-www-form-urlencoded").SetBody(form.Encode()).Post(pc.TokenURL)
	if err != nil {
		return "", "", err
	}
	if resp.StatusCode() < 200 || resp.StatusCode() >= 300 {
		return "", "", fmt.Errorf("keycloak: token endpoint returned %d", resp.StatusCode())
	}
	// naive parse to find access_token
	v := extractJSONField(resp.Body(), "access_token")
	if strings.TrimSpace(v) == "" {
		return "", "", errors.New("keycloak: access_token not found in response")
	}
	return headerOrDefault(pc.Header), "Bearer " + v, nil
}

func acquirePocketBase(ctx context.Context, pc ProviderConfig) (string, string, error) {
	if strings.TrimSpace(pc.BaseURL) == "" || strings.TrimSpace(pc.Email) == "" || strings.TrimSpace(pc.Password) == "" {
		return "", "", errors.New("pocketbase: base_url, email and password are required")
	}
	loginURL := strings.TrimRight(pc.BaseURL, "/") + "/api/admins/auth-with-password"
	body := map[string]string{"identity": pc.Email, "password": pc.Password}
	resp, err := resty.New().R().SetContext(ctx).SetHeader("Content-Type", "application/json").SetBody(body).Post(loginURL)
	if err != nil {
		return "", "", err
	}
	if resp.StatusCode() < 200 || resp.StatusCode() >= 300 {
		return "", "", fmt.Errorf("pocketbase: login returned %d", resp.StatusCode())
	}
	// naive parse to find token
	v := extractJSONField(resp.Body(), "token")
	if strings.TrimSpace(v) == "" {
		return "", "", errors.New("pocketbase: token not found in response")
	}
	return headerOrDefault(pc.Header), v, nil
}
