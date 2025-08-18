package auth

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/go-resty/resty/v2"
	"github.com/markbates/goth"
)

// Config represents the top-level auth configuration (e.g., from a global YAML).
// It is intentionally simple to avoid coupling to YAML parser; callers can fill this struct.
//
// Example mapping from examples/global.yaml:
//
//	auth.provider.name: google
//	auth.provider.id: abcd
//	auth.provider.password: asdfd
//	auth.provider.token_header: Authorization
//
// For keycloak (password grant):
//
//	name: keycloak
//	base_url: https://kc.example.com
//	realm: myrealm
//	client_id: myclient
//	client_secret: (optional)
//	username: alice
//	password: secret
//	token_header: Authorization (default if empty)
//
// For pocketbase (users):
//
//	name: pocketbase
//	base_url: https://pb.example.com
//	identity: user@example.com (or username)
//	password: secret
//	is_admin: false (default)
//
// For pocketbase (admins): set is_admin: true
//
// For goth-backed providers (interactive OAuth flows), you may register a goth.Provider
// externally and use AcquireTokenWithGoth; see function docs below.
type Config struct {
	Provider ProviderConfig
}

// ProviderConfig holds provider-specific data. Many fields are optional and only
// used by specific providers.
type ProviderConfig struct {
	Name        string
	TokenHeader string

	// Generic/OIDC
	ClientID     string
	ClientSecret string
	RedirectURL  string
	Scopes       []string
	Domain       string // e.g., for some providers

	// Keycloak
	BaseURL  string // e.g., https://keycloak.example.com
	Realm    string
	Username string
	Password string

	// PocketBase
	Identity string // email/username
	IsAdmin  bool   // use admin endpoint if true
}

// AcquireToken determines provider by name and returns (headerName, tokenValue).
// The headerName defaults to "Authorization" if TokenHeader is empty.
func AcquireToken(ctx context.Context, cfg Config) (string, string, error) {
	name := strings.ToLower(strings.TrimSpace(cfg.Provider.Name))
	if name == "" {
		return "", "", errors.New("provider name is required")
	}
	switch name {
	case "keycloak":
		return acquireKeycloak(ctx, cfg.Provider)
	case "pocketbase":
		return acquirePocketBase(ctx, cfg.Provider)
	default:
		// Attempt via goth hook (interactive or pre-established session). This is a no-op
		// unless the caller has set up Goth and provided a concrete provider/Session.
		return AcquireTokenWithGoth(ctx, cfg.Provider)
	}
}

// AcquireTokenWithGoth attempts to use markbates/goth to obtain an access token.
// This function requires that the caller has already registered a goth.Provider
// (via goth.UseProviders) and is able to supply a valid goth.Session for the provider
// (usually created during an OAuth redirect/callback flow). Since this is a library
// without an HTTP server, we provide a minimal scaffolding that looks up the provider
// and returns a clear error instructing how to proceed if a session isn't available.
//
// If you have a valid Session, you can call:
//
//	p, _ := goth.GetProvider(providerName)
//	sess, _ := p.UnmarshalSession(serializedSession)
//	user, err := p.FetchUser(sess)
//	token := user.AccessToken
//
// and then set that into your Env where RequestSpec can inject it.
func AcquireTokenWithGoth(_ context.Context, pc ProviderConfig) (string, string, error) {
	providerName := strings.TrimSpace(pc.Name)
	header := headerOrDefault(pc.TokenHeader)
	if providerName == "" {
		return header, "", errors.New("goth: provider name is empty")
	}
	if _, err := goth.GetProvider(providerName); err != nil {
		return header, "", fmt.Errorf("goth: provider %s is not registered: %w", providerName, err)
	}
	// We cannot complete the auth flow here without an HTTP redirect/callback.
	// Return a descriptive error to guide the integrator.
	return header, "", fmt.Errorf("goth: interactive OAuth flow required for provider %s; register provider and complete session externally to obtain AccessToken", providerName)
}

// acquireKeycloak performs Resource Owner Password Credentials grant against Keycloak.
// It returns the access_token value. If TokenHeader is empty, Authorization with Bearer is assumed.
func acquireKeycloak(ctx context.Context, pc ProviderConfig) (string, string, error) {
	if pc.BaseURL == "" || pc.Realm == "" || pc.ClientID == "" || pc.Username == "" || pc.Password == "" {
		return "", "", errors.New("keycloak: base_url, realm, client_id, username, and password are required")
	}
	endpoint := strings.TrimRight(pc.BaseURL, "/") +
		"/realms/" + url.PathEscape(pc.Realm) +
		"/protocol/openid-connect/token"

	form := map[string]string{
		"grant_type": "password",
		"client_id":  pc.ClientID,
		"username":   pc.Username,
		"password":   pc.Password,
	}
	if pc.ClientSecret != "" {
		form["client_secret"] = pc.ClientSecret
	}

	client := resty.New()
	resp, err := client.R().
		SetContext(ctx).
		SetHeader("Content-Type", "application/x-www-form-urlencoded").
		SetFormData(form).
		Post(endpoint)
	if err != nil {
		return "", "", err
	}
	if resp.StatusCode() < 200 || resp.StatusCode() >= 300 {
		return "", "", fmt.Errorf("keycloak: token endpoint failed: %d: %s", resp.StatusCode(), string(resp.Body()))
	}
	// Minimal JSON parsing without struct to avoid new deps; search for access_token
	access := extractJSONField(resp.Body(), "access_token")
	if access == "" {
		return "", "", errors.New("keycloak: access_token not found in response")
	}
	header := headerOrDefault(pc.TokenHeader)
	// If header is Authorization and value does not start with Bearer, prefix it.
	if strings.EqualFold(header, "authorization") && !strings.HasPrefix(strings.ToLower(access), "bearer ") {
		return header, "Bearer " + access, nil
	}
	return header, access, nil
}

// acquirePocketBase authenticates with PocketBase via password and returns the token.
// Defaults to users endpoint; if IsAdmin is true uses admins endpoint.
func acquirePocketBase(ctx context.Context, pc ProviderConfig) (string, string, error) {
	if pc.BaseURL == "" || pc.Identity == "" || pc.Password == "" {
		return "", "", errors.New("pocketbase: base_url, identity, and password are required")
	}
	base := strings.TrimRight(pc.BaseURL, "/")
	path := "/api/users/auth-with-password"
	if pc.IsAdmin {
		path = "/api/admins/auth-with-password"
	}
	endpoint := base + path

	type reqBody struct {
		Identity string `json:"identity"`
		Password string `json:"password"`
	}
	body := reqBody{Identity: pc.Identity, Password: pc.Password}

	client := resty.New()
	resp, err := client.R().
		SetContext(ctx).
		SetHeader("Content-Type", "application/json").
		SetBody(body).
		Post(endpoint)
	if err != nil {
		return "", "", err
	}
	if resp.StatusCode() < 200 || resp.StatusCode() >= 300 {
		return "", "", fmt.Errorf("pocketbase: auth failed: %d: %s", resp.StatusCode(), string(resp.Body()))
	}
	// PocketBase response includes a top-level token field
	token := extractJSONField(resp.Body(), "token")
	if token == "" {
		return "", "", errors.New("pocketbase: token not found in response")
	}
	header := headerOrDefault(pc.TokenHeader)
	if strings.EqualFold(header, "authorization") && !strings.HasPrefix(strings.ToLower(token), "bearer ") {
		return header, "Bearer " + token, nil
	}
	return header, token, nil
}
