package apimigrate

import (
	"context"
)

// Public, type-safe wrappers for built-in auth providers.
// These allow external library users to configure providers without using map[string]any.

// BasicAuthConfig mirrors the internal basic.Config.
// Header defaults to "Authorization" when empty.
type BasicAuthConfig struct {
	Name     string
	Header   string
	Username string
	Password string
}

// AcquireBasicAuth acquires and stores a Basic auth token under cfg.Name.
// Returns the header, value, and stored logical name.
func AcquireBasicAuth(ctx context.Context, cfg BasicAuthConfig) (string, string, string, error) {
	// Build provider spec expected by the internal registry
	spec := map[string]interface{}{
		"name":     cfg.Name,
		"header":   cfg.Header,
		"username": cfg.Username,
		"password": cfg.Password,
	}
	return AcquireAuthByProviderSpec(ctx, "basic", spec)
}

// OAuth2PasswordConfig mirrors fields for the OAuth2 Resource Owner Password Credentials grant.
// ClientSec is the client_secret. AuthURL is optional for password grant; TokenURL required.
// Scopes is optional.
type OAuth2PasswordConfig struct {
	Name      string
	Header    string
	ClientID  string
	ClientSec string
	AuthURL   string
	TokenURL  string
	Username  string
	Password  string
	Scopes    []string
}

// AcquireOAuth2Password acquires and stores an OAuth2 password-grant token under cfg.Name.
func AcquireOAuth2Password(ctx context.Context, cfg OAuth2PasswordConfig) (string, string, string, error) {
	sub := map[string]interface{}{
		"name":          cfg.Name,
		"header":        cfg.Header,
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
		"name":         cfg.Name,
		"grant_type":   "password",
		"grant_config": sub,
	}
	return AcquireAuthByProviderSpec(ctx, "oauth2", spec)
}

// OAuth2ClientCredentialsConfig mirrors fields for the OAuth2 Client Credentials grant.
// Scopes is optional.
type OAuth2ClientCredentialsConfig struct {
	Name      string
	Header    string
	ClientID  string
	ClientSec string
	TokenURL  string
	Scopes    []string
}

// AcquireOAuth2ClientCredentials acquires and stores a client-credentials token under cfg.Name.
func AcquireOAuth2ClientCredentials(ctx context.Context, cfg OAuth2ClientCredentialsConfig) (string, string, string, error) {
	sub := map[string]interface{}{
		"name":          cfg.Name,
		"header":        cfg.Header,
		"client_id":     cfg.ClientID,
		"client_secret": cfg.ClientSec,
		"token_url":     cfg.TokenURL,
	}
	if len(cfg.Scopes) > 0 {
		sub["scopes"] = cfg.Scopes
	}
	spec := map[string]interface{}{
		"name":         cfg.Name,
		"grant_type":   "client_credentials",
		"grant_config": sub,
	}
	return AcquireAuthByProviderSpec(ctx, "oauth2", spec)
}

// OAuth2ImplicitConfig mirrors fields for the OAuth2 Implicit grant.
// Acquire returns the header and a URL containing response_type=token so you can complete the flow externally.
// Scopes is optional.
type OAuth2ImplicitConfig struct {
	Name        string
	Header      string
	ClientID    string
	RedirectURL string
	AuthURL     string
	Scopes      []string
}

// AcquireOAuth2Implicit prepares the implicit grant authorization URL value under cfg.Name.
func AcquireOAuth2Implicit(ctx context.Context, cfg OAuth2ImplicitConfig) (string, string, string, error) {
	sub := map[string]interface{}{
		"name":         cfg.Name,
		"header":       cfg.Header,
		"client_id":    cfg.ClientID,
		"redirect_url": cfg.RedirectURL,
		"auth_url":     cfg.AuthURL,
	}
	if len(cfg.Scopes) > 0 {
		sub["scopes"] = cfg.Scopes
	}
	spec := map[string]interface{}{
		"name":         cfg.Name,
		"grant_type":   "implicit",
		"grant_config": sub,
	}
	return AcquireAuthByProviderSpec(ctx, "oauth2", spec)
}

// PocketBaseAuthConfig mirrors the internal pocketbase.Config.
// Header defaults to Authorization when empty.
type PocketBaseAuthConfig struct {
	Name     string
	Header   string
	BaseURL  string
	Email    string
	Password string
}

// AcquirePocketBase acquires and stores the PocketBase admin token under cfg.Name.
func AcquirePocketBase(ctx context.Context, cfg PocketBaseAuthConfig) (string, string, string, error) {
	spec := map[string]interface{}{
		"name":     cfg.Name,
		"header":   cfg.Header,
		"base_url": cfg.BaseURL,
		"email":    cfg.Email,
		"password": cfg.Password,
	}
	return AcquireAuthByProviderSpec(ctx, "pocketbase", spec)
}

// Below are convenience variants that accept an explicit logical name argument
// so callers don't need to embed the name into the config/spec.

// AcquireBasicAuthWithName acquires and stores a Basic auth token under the provided name.
// cfg.Name is ignored; pass the desired logical name via the name parameter.
func AcquireBasicAuthWithName(ctx context.Context, name string, cfg BasicAuthConfig) (string, string, string, error) {
	spec := map[string]interface{}{
		"header":   cfg.Header,
		"username": cfg.Username,
		"password": cfg.Password,
	}
	return AcquireAuthByProviderSpecWithName(ctx, "basic", name, spec)
}

// AcquireOAuth2PasswordWithName acquires and stores an OAuth2 password-grant token under the provided name.
// cfg.Name is ignored; pass the desired logical name via the name parameter.
func AcquireOAuth2PasswordWithName(ctx context.Context, name string, cfg OAuth2PasswordConfig) (string, string, string, error) {
	sub := map[string]interface{}{
		"header":        cfg.Header,
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
	return AcquireAuthByProviderSpecWithName(ctx, "oauth2", name, spec)
}

// AcquireOAuth2ClientCredentialsWithName acquires and stores a client-credentials token under the provided name.
// cfg.Name is ignored; pass the desired logical name via the name parameter.
func AcquireOAuth2ClientCredentialsWithName(ctx context.Context, name string, cfg OAuth2ClientCredentialsConfig) (string, string, string, error) {
	sub := map[string]interface{}{
		"header":        cfg.Header,
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
	return AcquireAuthByProviderSpecWithName(ctx, "oauth2", name, spec)
}

// AcquireOAuth2ImplicitWithName prepares the implicit grant authorization URL value under the provided name.
// cfg.Name is ignored; pass the desired logical name via the name parameter.
func AcquireOAuth2ImplicitWithName(ctx context.Context, name string, cfg OAuth2ImplicitConfig) (string, string, string, error) {
	sub := map[string]interface{}{
		"header":       cfg.Header,
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
	return AcquireAuthByProviderSpecWithName(ctx, "oauth2", name, spec)
}

// AcquirePocketBaseWithName acquires and stores the PocketBase admin token under the provided name.
// cfg.Name is ignored; pass the desired logical name via the name parameter.
func AcquirePocketBaseWithName(ctx context.Context, name string, cfg PocketBaseAuthConfig) (string, string, string, error) {
	spec := map[string]interface{}{
		"header":   cfg.Header,
		"base_url": cfg.BaseURL,
		"email":    cfg.Email,
		"password": cfg.Password,
	}
	return AcquireAuthByProviderSpecWithName(ctx, "pocketbase", name, spec)
}
