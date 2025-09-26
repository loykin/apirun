package auth

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/go-viper/mapstructure/v2"
	"github.com/loykin/apirun/internal/auth/basic"
	acommon "github.com/loykin/apirun/internal/auth/common"
	"github.com/loykin/apirun/internal/auth/oauth2"
	"github.com/loykin/apirun/internal/auth/pocketbase"
	"github.com/loykin/apirun/internal/common"
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
	logger := common.GetLogger().WithComponent("auth-registry")
	logger.Debug("acquiring authentication token", "provider_type", typ)

	f, ok := providers[normalizeKey(typ)]
	if !ok {
		logger.Error("unsupported auth provider type", "provider_type", typ)
		return "", errors.New("auth: unsupported provider type: " + typ)
	}
	m, err := f(spec)
	if err != nil {
		logger.Error("failed to create auth method", "error", err, "provider_type", typ)
		return "", fmt.Errorf("failed to create auth method for provider %q: %w", typ, err)
	}
	if ctx == nil {
		ctx = context.Background()
	}
	token, err := m.Acquire(ctx)
	if err != nil {
		logger.Error("failed to acquire auth token", "error", err, "provider_type", typ)
		return "", fmt.Errorf("failed to acquire auth token from provider %q: %w", typ, err)
	}

	logger.Info("authentication token acquired successfully", "provider_type", typ)
	return token, nil
}

// Built-in provider registrations
func init() {
	// oauth2 (and common aliases)
	Register(acommon.AuthTypeOAuth2, func(spec map[string]interface{}) (Method, error) {
		var c oauth2.Auth2Config
		if err := mapstructure.Decode(spec, &c); err != nil {
			return nil, fmt.Errorf("failed to decode OAuth2 configuration: %w", err)
		}
		m, err := c.GetGrantMethod()
		if err != nil {
			return nil, fmt.Errorf("failed to get OAuth2 grant method: %w", err)
		}
		return oauth2.Adapter{M: m}, nil
	})

	// basic
	Register(acommon.AuthTypeBasic, func(spec map[string]interface{}) (Method, error) {
		var c basic.Config
		if err := mapstructure.Decode(spec, &c); err != nil {
			return nil, fmt.Errorf("failed to decode basic auth configuration: %w", err)
		}
		return basic.Adapter{C: c}, nil
	})

	// pocketbase
	Register(acommon.AuthTypePocketBase, func(spec map[string]interface{}) (Method, error) {
		var c pocketbase.Config
		if err := mapstructure.Decode(spec, &c); err != nil {
			return nil, fmt.Errorf("failed to decode PocketBase auth configuration: %w", err)
		}
		return pocketbase.Adapter{C: c}, nil
	})
}
