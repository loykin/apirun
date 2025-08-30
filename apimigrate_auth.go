package apimigrate

import (
	"context"

	iauth "github.com/loykin/apimigrate/internal/auth"
	"github.com/loykin/apimigrate/internal/auth/basic"
	"github.com/loykin/apimigrate/internal/auth/common"
	"github.com/loykin/apimigrate/internal/auth/oauth2"
	"github.com/loykin/apimigrate/internal/auth/pocketbase"
)

// Public constants for known auth provider types usable with AcquireAuthAndSetEnv typ parameter.
// These map to the built-in registry keys. Custom providers can use their own type strings.
const (
	AuthTypeBasic      = common.AuthTypeBasic
	AuthTypeOAuth2     = common.AuthTypeOAuth2
	AuthTypePocketBase = common.AuthTypePocketBase
)

// Public, type-safe wrappers for built-in auth providers.
// These allow external library users to configure providers without using map[string]any.

// BasicAuthConfig mirrors the internal basic.Config.
// Header defaults to "Authorization" when empty.
type BasicAuthConfig basic.Config

func (b BasicAuthConfig) ToMap() map[string]interface{} {
	return basic.Config(b).ToMap()
}

// OAuth2PasswordConfig mirrors fields for the OAuth2 Resource Owner Password Credentials grant.
// ClientSec is the client_secret. AuthURL is optional for password grant; TokenURL required.
// Scopes is optional.
type OAuth2PasswordConfig oauth2.PasswordConfig

func (c OAuth2PasswordConfig) ToMap() map[string]interface{} {
	return oauth2.PasswordConfig(c).ToMap()
}

// OAuth2ClientCredentialsConfig mirrors fields for the OAuth2 Client Credentials grant.
// Scopes is optional.
type OAuth2ClientCredentialsConfig oauth2.ClientCredentialsConfig

func (c OAuth2ClientCredentialsConfig) ToMap() map[string]interface{} {
	return oauth2.ClientCredentialsConfig(c).ToMap()
}

// OAuth2ImplicitConfig mirrors fields for the OAuth2 Implicit grant.
// Acquire returns the header and a URL containing response_type=token so you can complete the flow externally.
// Scopes is optional.
type OAuth2ImplicitConfig oauth2.ImplicitConfig

func (c OAuth2ImplicitConfig) ToMap() map[string]interface{} {
	return oauth2.ImplicitConfig(c).ToMap()
}

type PocketBaseAuthConfig pocketbase.Config

func (c PocketBaseAuthConfig) ToMap() map[string]interface{} {
	return pocketbase.Config(c).ToMap()
}

// Below are convenience variants that accept an explicit logical name argument
// so callers don't need to embed the name into the config/spec.

// AcquireBasicAuthWithName acquires and stores a Basic auth token under the provided name.
// cfg.Name is ignored; pass the desired logical name via the name parameter.
func AcquireBasicAuthWithName(ctx context.Context, cfg BasicAuthConfig) (string, error) {
	spec := map[string]interface{}{
		"username": cfg.Username,
		"password": cfg.Password,
	}
	v, err := iauth.AcquireAndStoreWithName(ctx, "basic", spec)
	return v, err
}

// AcquireOAuth2PasswordWithName acquires and stores an OAuth2 password-grant token under the provided name.
// cfg.Name is ignored; pass the desired logical name via the name parameter.
func AcquireOAuth2PasswordWithName(ctx context.Context, cfg OAuth2PasswordConfig) (string, error) {
	sub := map[string]interface{}{
		"client_id":     cfg.ClientID,
		"client_secret": cfg.ClientSec,
		"auth_url":      cfg.AuthURL,
		"token_url":     cfg.TokenURL,
		"username":      cfg.Username,
		"password":      cfg.Password,
	}
	if len(cfg.Scopes) > 0 {
		sub["scopes"] = cfg.Scopes
	}
	spec := map[string]interface{}{
		"grant_type":   "password",
		"grant_config": sub,
	}
	v, err := iauth.AcquireAndStoreWithName(ctx, "oauth2", spec)
	return v, err
}

// AcquireOAuth2ClientCredentialsWithName acquires and stores a client-credentials token under the provided name.
// cfg.Name is ignored; pass the desired logical name via the name parameter.
func AcquireOAuth2ClientCredentialsWithName(ctx context.Context, cfg OAuth2ClientCredentialsConfig) (string, error) {
	sub := map[string]interface{}{
		"client_id":     cfg.ClientID,
		"client_secret": cfg.ClientSec,
		"token_url":     cfg.TokenURL,
	}
	if len(cfg.Scopes) > 0 {
		sub["scopes"] = cfg.Scopes
	}
	spec := map[string]interface{}{
		"grant_type":   "client_credentials",
		"grant_config": sub,
	}
	v, err := iauth.AcquireAndStoreWithName(ctx, "oauth2", spec)
	return v, err
}

// AcquireOAuth2ImplicitWithName prepares the implicit grant authorization URL value under the provided name.
// cfg.Name is ignored; pass the desired logical name via the name parameter.
func AcquireOAuth2ImplicitWithName(ctx context.Context, cfg OAuth2ImplicitConfig) (string, error) {
	sub := map[string]interface{}{
		"client_id":    cfg.ClientID,
		"redirect_url": cfg.RedirectURL,
		"auth_url":     cfg.AuthURL,
	}
	if len(cfg.Scopes) > 0 {
		sub["scopes"] = cfg.Scopes
	}
	spec := map[string]interface{}{
		"grant_type":   "implicit",
		"grant_config": sub,
	}
	v, err := iauth.AcquireAndStoreWithName(ctx, "oauth2", spec)
	return v, err
}

// AcquirePocketBaseWithName acquires and stores the PocketBase admin token under the provided name.
// cfg.Name is ignored; pass the desired logical name via the name parameter.
func AcquirePocketBaseWithName(ctx context.Context, cfg PocketBaseAuthConfig) (string, error) {
	spec := map[string]interface{}{
		"base_url": cfg.BaseURL,
		"email":    cfg.Email,
		"password": cfg.Password,
	}
	v, err := iauth.AcquireAndStoreWithName(ctx, "pocketbase", spec)
	return v, err
}
