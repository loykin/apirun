package basic

import (
	"encoding/base64"
	"errors"
	"strings"

	"github.com/loykin/apimigrate/internal/auth/common"
)

// Config holds configuration for Basic authentication.
type Config struct {
	Name     string `mapstructure:"name"`
	Header   string `mapstructure:"header"`
	Username string `mapstructure:"username"`
	Password string `mapstructure:"password"`
}

// AcquireBasic returns a Basic auth header value constructed from Username and Password.
// Header defaults to Authorization when empty.
func AcquireBasic(pc Config) (string, string, error) {
	u := strings.TrimSpace(pc.Username)
	p := strings.TrimSpace(pc.Password)
	if u == "" || p == "" {
		return "", "", errors.New("basic: username and password are required")
	}
	cred := base64.StdEncoding.EncodeToString([]byte(u + ":" + p))
	return common.HeaderOrDefault(pc.Header), "Basic " + cred, nil
}
