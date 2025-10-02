# Authentication Guide

Complete guide for configuring and using authentication providers in apirun.

## Overview

apirun supports multiple authentication providers that can be used in migrations to authenticate HTTP requests:

- **Basic Authentication**: Username/password authentication
- **OAuth2**: Various OAuth2 grant types (client credentials, authorization code)
- **PocketBase**: PocketBase-specific authentication
- **Custom Providers**: Register your own authentication providers

## Built-in Authentication Providers

### Basic Authentication

Simple username/password authentication.

#### Configuration

```yaml
auth:
  - type: basic
    name: api_basic
    config:
      username: admin
      password: "{{.admin_password}}"  # supports templating
```

#### Usage in Migrations

```yaml
up:
  name: create user
  request:
    auth_name: api_basic  # references the auth provider name
    method: POST
    url: "{{.api_base}}/users"
    headers:
      - name: Content-Type
        value: application/json
    body: |
      {"username": "demo", "enabled": true}
```

### OAuth2 Authentication

Support for various OAuth2 grant types.

#### Client Credentials Grant

```yaml
auth:
  - type: oauth2
    name: api_oauth
    config:
      client_id: "{{.oauth_client_id}}"
      client_secret: "{{.oauth_client_secret}}"
      token_url: "https://auth.example.com/oauth/token"
      grant_type: client_credentials
      scopes: ["read", "write"]  # optional
```

#### Authorization Code Grant

```yaml
auth:
  - type: oauth2
    name: github_oauth
    config:
      client_id: "{{.github_client_id}}"
      client_secret: "{{.github_client_secret}}"
      token_url: "https://github.com/login/oauth/access_token"
      grant_type: authorization_code
      authorization_url: "https://github.com/login/oauth/authorize"
      redirect_uri: "http://localhost:8080/callback"
      scopes: ["user:email", "repo"]
```

#### Password Grant

```yaml
auth:
  - type: oauth2
    name: user_oauth
    config:
      client_id: "{{.oauth_client_id}}"
      client_secret: "{{.oauth_client_secret}}"
      token_url: "{{.oauth_token_url}}"
      grant_type: password
      username: "{{.user_username}}"
      password: "{{.user_password}}"
      scopes: ["api"]
```

### PocketBase Authentication

PocketBase-specific authentication with admin or user credentials.

#### Admin Authentication

```yaml
auth:
  - type: pocketbase
    name: pb_admin
    config:
      base_url: "{{.pocketbase_url}}"
      username: "{{.pb_admin_email}}"
      password: "{{.pb_admin_password}}"
      type: admin  # optional, defaults to admin
```

#### User Authentication

```yaml
auth:
  - type: pocketbase
    name: pb_user
    config:
      base_url: "{{.pocketbase_url}}"
      username: "{{.pb_user_email}}"
      password: "{{.pb_user_password}}"
      type: user
```

## Using Authentication in Migrations

### Basic Usage

```yaml
up:
  name: authenticated request
  request:
    auth_name: api_basic  # references auth provider
    method: GET
    url: "{{.api_base}}/protected-resource"
  response:
    result_code: ["200"]
```

### Explicit Header Usage

apirun never auto-prefixes Authorization headers. You must set headers explicitly:

```yaml
up:
  name: explicit auth header
  request:
    auth_name: api_oauth
    method: POST
    url: "{{.api_base}}/api/v1/resource"
    headers:
      - name: Authorization
        value: "Bearer {{.auth.api_oauth}}"  # use acquired token
      - name: Content-Type
        value: application/json
    body: |
      {"data": "example"}
```

### Multiple Auth Providers

```yaml
# Configuration with multiple providers
auth:
  - type: basic
    name: admin_auth
    config:
      username: admin
      password: "{{.admin_password}}"

  - type: oauth2
    name: user_auth
    config:
      client_id: "{{.oauth_client_id}}"
      client_secret: "{{.oauth_client_secret}}"
      token_url: "{{.oauth_token_url}}"

# Migration using specific provider
up:
  name: admin operation
  request:
    auth_name: admin_auth  # uses basic auth
    method: POST
    url: "{{.api_base}}/admin/users"
```

## Programmatic Authentication (Library)

### Embedded Authentication

Automatic token acquisition during migration execution:

```go
package main

import (
    "context"
    "github.com/loykin/apirun"
)

func main() {
    ctx := context.Background()

    // Configure environment
    env := apirun.Env{
        Global: map[string]string{
            "api_base": "http://localhost:8080",
            "admin_password": "secret123",
        },
    }

    // Configure authentication
    auth := apirun.Auth{
        Type: apirun.AuthTypeBasic,
        Name: "api_basic",
        Methods: apirun.BasicAuthConfig{
            Username: "admin",
            Password: "{{.admin_password}}",
        },
    }

    // Setup migrator with auth
    migrator := apirun.Migrator{
        Dir: "./migrations",
        Env: env,
        Auth: []apirun.Auth{auth},
    }

    // Execute migrations - auth tokens acquired automatically
    results, err := migrator.MigrateUp(ctx, 0)
    if err != nil {
        panic(err)
    }
}
```

### Multiple Embedded Providers

```go
func main() {
    ctx := context.Background()

    env := apirun.Env{
        Global: map[string]string{
            "api_base": "http://localhost:8080",
            "admin_password": "secret123",
            "oauth_client_id": "client123",
            "oauth_client_secret": "secret456",
        },
    }

    // Configure multiple auth providers
    basicAuth := apirun.Auth{
        Type: apirun.AuthTypeBasic,
        Name: "admin_basic",
        Methods: apirun.BasicAuthConfig{
            Username: "admin",
            Password: "{{.admin_password}}",
        },
    }

    oauthAuth := apirun.Auth{
        Type: apirun.AuthTypeOAuth2,
        Name: "api_oauth",
        Methods: apirun.OAuth2Config{
            ClientID:     "{{.oauth_client_id}}",
            ClientSecret: "{{.oauth_client_secret}}",
            TokenURL:     "{{.oauth_token_url}}",
            GrantType:    "client_credentials",
        },
    }

    migrator := apirun.Migrator{
        Dir: "./migrations",
        Env: env,
        Auth: []apirun.Auth{basicAuth, oauthAuth},
    }

    results, err := migrator.MigrateUp(ctx, 0)
    // Both auth providers will acquire tokens automatically
}
```

### Decoupled Authentication

Acquire tokens first, then run migrations:

```go
func main() {
    ctx := context.Background()

    env := apirun.Env{
        Global: map[string]string{
            "api_base": "http://localhost:8080",
        },
    }

    // Configure auth provider
    auth := &apirun.Auth{
        Type: apirun.AuthTypeBasic,
        Name: "basic",
        Methods: apirun.BasicAuthConfig{
            Username: "admin",
            Password: "secret123",
        },
    }

    // Acquire token manually
    token, err := auth.Acquire(ctx, &env)
    if err != nil {
        panic(err)
    }

    // Add token to environment
    if env.Auth == nil {
        env.Auth = map[string]string{}
    }
    env.Auth["basic"] = token

    // Run migrations with pre-acquired tokens
    migrator := apirun.Migrator{
        Dir: "./migrations",
        Env: env,
    }

    results, err := migrator.MigrateUp(ctx, 0)
}
```

### Lazy Authentication

Acquire tokens only when first used:

```go
func main() {
    ctx := context.Background()

    env := apirun.Env{
        Global: map[string]string{
            "api_base": "http://localhost:8080",
        },
    }

    // Configure auth with lazy acquisition
    auth := apirun.Auth{
        Type: apirun.AuthTypeBasic,
        Name: "lazy_basic",
        Methods: apirun.BasicAuthConfig{
            Username: "admin",
            Password: "secret123",
        },
        LazyAcquisition: true,  // Acquire only when needed
    }

    migrator := apirun.Migrator{
        Dir: "./migrations",
        Env: env,
        Auth: []apirun.Auth{auth},
    }

    // Token acquired only when first migration uses lazy_basic
    results, err := migrator.MigrateUp(ctx, 0)
}
```

## Custom Authentication Providers

### Registering Custom Providers

```go
package main

import (
    "context"
    "fmt"
    "github.com/loykin/apirun"
)

// Custom auth method implementation
type CustomAuthMethod struct {
    APIKey string
    Secret string
}

func (c *CustomAuthMethod) Acquire(ctx context.Context, env *apirun.Env) (string, error) {
    // Implement your custom authentication logic
    // Return the token/credential to use in requests
    token := fmt.Sprintf("custom_%s_%s", c.APIKey, c.Secret)
    return token, nil
}

func main() {
    // Register custom auth provider
    apirun.RegisterAuthProvider("custom", func(spec map[string]interface{}) (apirun.AuthMethod, error) {
        apiKey, ok := spec["api_key"].(string)
        if !ok {
            return nil, fmt.Errorf("api_key is required")
        }

        secret, ok := spec["secret"].(string)
        if !ok {
            return nil, fmt.Errorf("secret is required")
        }

        return &CustomAuthMethod{
            APIKey: apiKey,
            Secret: secret,
        }, nil
    })

    // Use custom provider
    ctx := context.Background()
    token, err := apirun.AcquireAuthByProviderSpecWithName(
        ctx,
        "custom",           // provider type
        "my-custom",        // logical name
        map[string]interface{}{
            "api_key": "key123",
            "secret":  "secret456",
        },
    )

    fmt.Printf("Acquired token: %s\n", token)
}
```

### Using Custom Providers in Configuration

```yaml
# config.yaml
auth:
  - type: custom
    name: my_custom_auth
    config:
      api_key: "{{.custom_api_key}}"
      secret: "{{.custom_secret}}"

env:
  - name: custom_api_key
    valueFromEnv: CUSTOM_API_KEY
  - name: custom_secret
    valueFromEnv: CUSTOM_SECRET
```

## Template Variables and Token Access

### Accessing Auth Tokens in Templates

Authentication tokens are available in templates under the `auth` namespace:

```yaml
up:
  name: use auth token
  request:
    method: GET
    url: "{{.api_base}}/protected"
    headers:
      - name: Authorization
        value: "Bearer {{.auth.api_oauth}}"  # token from api_oauth provider
      - name: X-API-Key
        value: "{{.auth.custom_auth}}"       # token from custom_auth provider
```

### Variable Namespaces

- `{{.env.variable}}`: Environment variables
- `{{.auth.provider_name}}`: Authentication tokens
- `{{.variable}}`: Direct access (legacy, prefer namespaced access)

## Authentication Best Practices

### 1. Use Environment Variables for Secrets

```yaml
# Good: Use environment variables
auth:
  - type: basic
    name: api_auth
    config:
      username: "{{.api_username}}"
      password: "{{.api_password}}"

env:
  - name: api_username
    valueFromEnv: API_USERNAME
  - name: api_password
    valueFromEnv: API_PASSWORD
```

```yaml
# Bad: Hardcode secrets
auth:
  - type: basic
    name: api_auth
    config:
      username: admin
      password: secret123  # Don't do this!
```

### 2. Use Descriptive Auth Names

```yaml
# Good: Descriptive names
auth:
  - type: basic
    name: admin_api_auth
  - type: oauth2
    name: user_service_oauth

# Bad: Generic names
auth:
  - type: basic
    name: auth1
  - type: oauth2
    name: auth2
```

### 3. Explicit Authorization Headers

```yaml
# Good: Explicit header management
headers:
  - name: Authorization
    value: "Bearer {{.auth.api_oauth}}"

# Bad: Expecting auto-prefixing (apirun doesn't do this)
# auth_name: api_oauth  # This won't automatically add Authorization header
```

### 4. Environment-Specific Auth

```yaml
# development.yaml
auth:
  - type: basic
    name: dev_auth
    config:
      username: dev
      password: dev123

# production.yaml
auth:
  - type: oauth2
    name: prod_auth
    config:
      client_id: "{{.prod_client_id}}"
      client_secret: "{{.prod_client_secret}}"
      token_url: "{{.prod_token_url}}"
```

## Troubleshooting Authentication

### Common Issues

#### Token Not Available in Template

```yaml
Error: template: executing "template" at <.auth.api_oauth>: map has no entry for key "api_oauth"

# Check auth provider name matches
auth:
  - name: api_oauth  # This name must match template reference
```

#### Authentication Failure

```yaml
Error: authentication failed for provider 'api_basic': 401 Unauthorized

# Check credentials and endpoint
auth:
  - type: basic
    name: api_basic
    config:
      username: "{{.correct_username}}"
      password: "{{.correct_password}}"
```

#### OAuth2 Token Acquisition Failed

```yaml
Error: oauth2 token acquisition failed: invalid_client

# Verify OAuth2 configuration
auth:
  - type: oauth2
    name: oauth_provider
    config:
      client_id: "{{.valid_client_id}}"
      client_secret: "{{.valid_client_secret}}"
      token_url: "https://correct-auth-server.com/oauth/token"
```

### Debugging Commands

```bash
# Test authentication without running migrations
apirun up --dry-run -v

# Check environment variable expansion
apirun up --dry-run | grep -A 5 "auth"

# Validate configuration
apirun validate
```

## Migration Examples

### Basic Auth Example

```yaml
# migration/001_create_user.yaml
up:
  name: create user with basic auth
  env:
    username: testuser
    email: test@example.com
  request:
    auth_name: admin_basic
    method: POST
    url: "{{.api_base}}/users"
    headers:
      - name: Content-Type
        value: application/json
    body: |
      {
        "username": "{{.username}}",
        "email": "{{.email}}",
        "enabled": true
      }
  response:
    result_code: ["201"]
    env_from:
      user_id: "id"

down:
  name: delete user
  auth: admin_basic
  method: DELETE
  url: "{{.api_base}}/users/{{.user_id}}"
```

### OAuth2 Example

```yaml
# migration/002_update_permissions.yaml
up:
  name: update user permissions with oauth2
  request:
    auth_name: user_oauth
    method: PATCH
    url: "{{.api_base}}/users/{{.user_id}}/permissions"
    headers:
      - name: Authorization
        value: "Bearer {{.auth.user_oauth}}"
      - name: Content-Type
        value: application/json
    body: |
      {
        "permissions": ["read", "write", "admin"]
      }
  response:
    result_code: ["200"]
```

### Multiple Auth Providers Example

```yaml
# migration/003_cross_service_operation.yaml
up:
  name: operation requiring multiple services
  env:
    operation_id: "op_{{.timestamp}}"
  request:
    # First request with admin auth
    auth_name: admin_basic
    method: POST
    url: "{{.admin_api_base}}/operations"
    body: |
      {
        "id": "{{.operation_id}}",
        "type": "cross_service"
      }
  response:
    result_code: ["201"]

# Second step with different auth
- name: notify user service
  request:
    auth_name: user_service_oauth
    method: POST
    url: "{{.user_api_base}}/notifications"
    headers:
      - name: Authorization
        value: "Bearer {{.auth.user_service_oauth}}"
      - name: Content-Type
        value: application/json
    body: |
      {
        "operation_id": "{{.operation_id}}",
        "message": "Operation completed"
      }
  response:
    result_code: ["200"]
```