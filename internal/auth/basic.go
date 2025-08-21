package auth

import (
	"encoding/base64"
	"errors"
	"strings"
)

// BasicConfig holds configuration for Basic authentication.
type BasicConfig struct {
	Name     string `mapstructure:"name"`
	Header   string `mapstructure:"header"`
	Username string `mapstructure:"username"`
	Password string `mapstructure:"password"`
}

// acquireBasic returns a Basic auth header value constructed from Username and Password.
// Header defaults to Authorization when empty.
func acquireBasic(pc BasicConfig) (string, string, error) {
	u := strings.TrimSpace(pc.Username)
	p := strings.TrimSpace(pc.Password)
	if u == "" || p == "" {
		return "", "", errors.New("basic: username and password are required")
	}
	cred := base64.StdEncoding.EncodeToString([]byte(u + ":" + p))
	return headerOrDefault(pc.Header), "Basic " + cred, nil
}
