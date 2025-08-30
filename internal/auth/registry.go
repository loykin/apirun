package auth

import (
	"context"
	"errors"
	"strings"

	"github.com/go-viper/mapstructure/v2"
	"github.com/loykin/apimigrate/internal/auth/basic"
	"github.com/loykin/apimigrate/internal/auth/common"
	"github.com/loykin/apimigrate/internal/auth/oauth2"
	"github.com/loykin/apimigrate/internal/auth/pocketbase"
)

// Method is the plugin interface for an authentication method.
// Implementations should be lightweight wrappers around configuration
// that can acquire a token value. Header handling is externalized.
// Acquire returns only the token value to inject (e.g., "Basic ..." or "Bearer ...").
type Method interface {
	Acquire(ctx context.Context) (value string, err error)
}

// Factory builds a Method instance from a loosely-typed spec map.
// Decoding into a concrete config struct is the typical responsibility of a Factory.
type Factory func(spec map[string]interface{}) (Method, error)

// In-memory registry of provider factories keyed by normalized type.
var providers = map[string]Factory{}

// normalizeKey lower-cases and trims provider type keys.
func normalizeKey(s string) string { return strings.ToLower(strings.TrimSpace(s)) }

// Register registers an auth provider factory under a type key (e.g., "oauth2", "basic").
// The key is normalized to lower-case.
func Register(typ string, f Factory) {
	key := normalizeKey(typ)
	if key == "" {
		return
	}
	if f == nil {
		return
	}
	providers[key] = f
}

// AcquireAndStoreWithName builds a Method from the provider type and spec and acquires a token.
// Note: name is no longer required and thus removed from the API since tokens are not stored globally anymore.
func AcquireAndStoreWithName(ctx context.Context, typ string, spec map[string]interface{}) (string, error) {
	f, ok := providers[normalizeKey(typ)]
	if !ok {
		return "", errors.New("auth: unsupported provider type: " + typ)
	}
	m, err := f(spec)
	if err != nil {
		return "", err
	}
	if ctx == nil {
		ctx = context.Background()
	}
	return m.Acquire(ctx)
}

// Built-in provider registrations
func init() {
	// oauth2 (and common aliases)
	Register(common.AuthTypeOAuth2, func(spec map[string]interface{}) (Method, error) {
		var c oauth2.Auth2Config
		if err := mapstructure.Decode(spec, &c); err != nil {
			return nil, err
		}
		m, err := c.GetGrantMethod()
		if err != nil {
			return nil, err
		}
		return oauth2.Adapter{M: m}, nil
	})

	// basic
	Register(common.AuthTypeBasic, func(spec map[string]interface{}) (Method, error) {
		var c basic.Config
		if err := mapstructure.Decode(spec, &c); err != nil {
			return nil, err
		}
		return basic.Adapter{C: c}, nil
	})

	// pocketbase
	Register(common.AuthTypePocketBase, func(spec map[string]interface{}) (Method, error) {
		var c pocketbase.Config
		if err := mapstructure.Decode(spec, &c); err != nil {
			return nil, err
		}
		return pocketbase.Adapter{C: c}, nil
	})
}
