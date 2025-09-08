package custom_jwt

import (
	"context"
	"fmt"

	"github.com/go-viper/mapstructure/v2"
	iauth "github.com/loykin/apimigrate/internal/auth"
)

const Type = "custom_jwt"

// Register into the global auth registry on init so users can Acquire tokens using apimigrate.RegisterAuthProvider or built-ins.
func init() {
	iauth.Register(Type, func(spec map[string]interface{}) (iauth.Method, error) {
		var c Config
		if err := mapstructure.Decode(spec, &c); err != nil {
			return nil, err
		}
		return method{cfg: c}, nil
	})
}

type method struct{ cfg Config }

func (m method) Acquire(ctx context.Context) (string, error) {
	_ = ctx
	tok, err := m.cfg.Issue()
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("Bearer %s", tok), nil
}
