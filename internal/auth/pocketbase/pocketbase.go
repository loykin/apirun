package pocketbase

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/go-resty/resty/v2"
	"github.com/loykin/apimigrate/internal/auth/common"
)

// Config holds configuration for PocketBase admin login.
type Config struct {
	Name     string `mapstructure:"name"`
	Header   string `mapstructure:"header"`
	BaseURL  string `mapstructure:"base_url"`
	Email    string `mapstructure:"email"`
	Password string `mapstructure:"password"`
}

func AcquirePocketBase(ctx context.Context, pc Config) (string, string, error) {
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
	// parse JSON response to get token
	type loginResp struct {
		Token string `json:"token"`
	}
	var lr loginResp
	if err := json.Unmarshal(resp.Body(), &lr); err != nil {
		return "", "", fmt.Errorf("pocketbase: invalid JSON response: %w", err)
	}
	v := strings.TrimSpace(lr.Token)
	if v == "" {
		return "", "", errors.New("pocketbase: token not found in response")
	}
	return common.HeaderOrDefault(pc.Header), v, nil
}
