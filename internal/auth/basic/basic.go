package basic

import (
	"encoding/base64"
	"fmt"

	"github.com/loykin/apirun/internal/util"
)

// Config holds configuration for Basic authentication.
type Config struct {
	Username string `mapstructure:"username"`
	Password string `mapstructure:"password"`
}

// ToMap implements apirun.AuthSpec-like mapping for internal use/consistency.
func (c Config) ToMap() map[string]interface{} {
	return map[string]interface{}{
		"username": c.Username,
		"password": c.Password,
	}
}

// AcquireBasic returns a Basic auth token constructed from Username and Password.
// It returns only the base64(username:password) token string (no "Basic " prefix).
func AcquireBasic(pc Config) (string, error) {
	username, hasUsername := util.TrimEmptyCheck(pc.Username)
	password, hasPassword := util.TrimEmptyCheck(pc.Password)
	if !hasUsername || !hasPassword {
		return "", fmt.Errorf("basic: username and password are required")
	}
	cred := base64.StdEncoding.EncodeToString([]byte(username + ":" + password))
	return cred, nil
}
