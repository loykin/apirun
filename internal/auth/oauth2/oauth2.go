package oauth2

import (
	"errors"
	"strings"

	"github.com/go-viper/mapstructure/v2"
)

type Auth2Config struct {
	GrantType   string                 `mapstructure:"grant_type"`
	GrantConfig map[string]interface{} `mapstructure:"grant_config"`
}

// GetGrantMethod builds and returns a grant-specific oauth2 Method using the
// GrantType and GrantConfig fields. This avoids the generic Build() path for simplicity.
func (c Auth2Config) GetGrantMethod() (Method, error) {
	gt := strings.ToLower(strings.TrimSpace(c.GrantType))
	if gt == "" {
		return nil, errors.New("auth: oauth2 grant_type is required")
	}
	if c.GrantConfig == nil {
		return nil, errors.New("auth: oauth2 grant_config is required")
	}
	sub := c.GrantConfig
	switch gt {
	case "password":
		var pc PasswordConfig
		if err := mapstructure.Decode(sub, &pc); err != nil {
			return nil, err
		}
		return passwordMethod{c: pc}, nil
	case "client_credentials", "client-credentials":
		var cc ClientCredentialsConfig
		if err := mapstructure.Decode(sub, &cc); err != nil {
			return nil, err
		}
		return clientCredentialsMethod{c: cc}, nil
	case "implicit":
		var ic ImplicitConfig
		if err := mapstructure.Decode(sub, &ic); err != nil {
			return nil, err
		}
		return implicitMethod{c: ic}, nil
	default:
		return nil, errors.New("auth: unsupported oauth2 grant_type: " + gt)
	}
}
