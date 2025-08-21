package auth

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/go-resty/resty/v2"
)

// PocketBaseConfig holds configuration for PocketBase admin login.
type PocketBaseConfig struct {
	Name     string `mapstructure:"name"`
	Header   string `mapstructure:"header"`
	BaseURL  string `mapstructure:"base_url"`
	Email    string `mapstructure:"email"`
	Password string `mapstructure:"password"`
}

func acquirePocketBase(ctx context.Context, pc PocketBaseConfig) (string, string, error) {
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
