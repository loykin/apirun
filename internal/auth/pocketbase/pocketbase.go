package pocketbase

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	acommon "github.com/loykin/apirun/internal/auth/common"
	"github.com/loykin/apirun/internal/httpc"
	"github.com/loykin/apirun/internal/util"
)

// Config holds configuration for PocketBase admin login.
type Config struct {
	BaseURL  string `mapstructure:"base_url"`
	Email    string `mapstructure:"email"`
	Password string `mapstructure:"password"`
}

// ToMap returns a spec map for the pocketbase provider.
func (c Config) ToMap() map[string]interface{} {
	return map[string]interface{}{
		"base_url": c.BaseURL,
		"email":    c.Email,
		"password": c.Password,
	}
}

func AcquirePocketBase(ctx context.Context, pc Config) (string, error) {
	baseURL, hasBaseURL := util.TrimEmptyCheck(pc.BaseURL)
	email, hasEmail := util.TrimEmptyCheck(pc.Email)
	password, hasPassword := util.TrimEmptyCheck(pc.Password)
	if !hasBaseURL || !hasEmail || !hasPassword {
		return "", fmt.Errorf("pocketbase: base_url, email and password are required")
	}
	loginURL := strings.TrimRight(baseURL, "/") + "/api/admins/auth-with-password"
	body := map[string]string{"identity": email, "password": password}
	h := httpc.Httpc{TlsConfig: acommon.GetTLSConfig()}
	client := h.New()
	resp, err := client.R().SetContext(ctx).SetHeader("Content-Type", "application/json").SetBody(body).Post(loginURL)
	if err != nil {
		return "", err
	}
	if resp.StatusCode() < 200 || resp.StatusCode() >= 300 {
		return "", fmt.Errorf("pocketbase: login returned %d", resp.StatusCode())
	}
	// parse JSON response to get token
	type loginResp struct {
		Token string `json:"token"`
	}
	var lr loginResp
	if err := json.Unmarshal(resp.Body(), &lr); err != nil {
		return "", fmt.Errorf("pocketbase: invalid JSON response: %w", err)
	}
	token, hasToken := util.TrimEmptyCheck(lr.Token)
	if !hasToken {
		return "", fmt.Errorf("pocketbase: token not found in response")
	}
	return token, nil
}
