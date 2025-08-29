package auth

import (
	"context"
	"errors"
	"strings"

	"github.com/go-viper/mapstructure/v2"
	"github.com/loykin/apimigrate/internal/auth/basic"
	"github.com/loykin/apimigrate/internal/auth/oauth2"
	"github.com/loykin/apimigrate/internal/auth/pocketbase"
)

// Method is the plugin interface for an authentication method.
// Implementations should be lightweight wrappers around configuration
// that can acquire a header/value pair.
// Name returns the logical token name used by RequestSpec.auth_name.
// Acquire returns the header name and header value to inject.
type Method interface {
	Name() string
	Acquire(ctx context.Context) (header string, value string, err error)
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

// AcquireFromMap builds a Method from the provider type and raw spec map and acquires a token.
// Returns header, value, and the logical name for storage.
func AcquireFromMap(ctx context.Context, typ string, spec map[string]interface{}) (string, string, string, error) {
	f, ok := providers[normalizeKey(typ)]
	if !ok {
		return "", "", "", errors.New("auth: unsupported provider type: " + typ)
	}
	m, err := f(spec)
	if err != nil {
		return "", "", "", err
	}
	// Ensure a non-nil context is passed to providers for safety
	if ctx == nil {
		ctx = context.Background()
	}
	h, v, err := m.Acquire(ctx)
	if err != nil {
		return "", "", "", err
	}
	name := strings.TrimSpace(m.Name())
	return h, v, name, nil
}

// AcquireAndStoreFromMap is like AcquireFromMap but also stores the token under the logical name.
func AcquireAndStoreFromMap(ctx context.Context, typ string, spec map[string]interface{}) (string, string, string, error) {
	h, v, name, err := AcquireFromMap(ctx, typ, spec)
	if err == nil && name != "" {
		SetToken(name, h, v)
	}
	return h, v, name, err
}

// AcquireAndStoreWithName acquires using the provider spec but stores under the provided name,
// ignoring the provider's Method.Name(). This allows callers to avoid including "name" in spec.
func AcquireAndStoreWithName(ctx context.Context, typ string, name string, spec map[string]interface{}) (string, string, string, error) {
	h, v, _, err := AcquireFromMap(ctx, typ, spec)
	stored := strings.TrimSpace(name)
	if err == nil && stored != "" {
		SetToken(stored, h, v)
	}
	return h, v, stored, err
}

// Built-in provider registrations
func init() {
	// oauth2 (and common aliases)
	Register("oauth2", func(spec map[string]interface{}) (Method, error) {
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
	Register("basic", func(spec map[string]interface{}) (Method, error) {
		var c basic.Config
		if err := mapstructure.Decode(spec, &c); err != nil {
			return nil, err
		}
		return basic.Adapter{C: c}, nil
	})

	// pocketbase
	Register("pocketbase", func(spec map[string]interface{}) (Method, error) {
		var c pocketbase.Config
		if err := mapstructure.Decode(spec, &c); err != nil {
			return nil, err
		}
		return pocketbase.Adapter{C: c}, nil
	})
}
