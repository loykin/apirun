package basic

import (
	"encoding/base64"
	"errors"
	"strings"
)

// Config holds configuration for Basic authentication.
type Config struct {
	Username string `mapstructure:"username"`
	Password string `mapstructure:"password"`
}

// ToMap implements apimigrate.AuthSpec-like mapping for internal use/consistency.
func (c Config) ToMap() map[string]interface{} {
	return map[string]interface{}{
		"username": c.Username,
		"password": c.Password,
	}
}

// AcquireBasic returns a Basic auth token constructed from Username and Password.
// It returns only the base64(username:password) token string (no "Basic " prefix).
func AcquireBasic(pc Config) (string, error) {
	u := strings.TrimSpace(pc.Username)
	p := strings.TrimSpace(pc.Password)
	if u == "" || p == "" {
		return "", errors.New("basic: username and password are required")
	}
	cred := base64.StdEncoding.EncodeToString([]byte(u + ":" + p))
	return cred, nil
}
