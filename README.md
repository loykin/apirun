# apimigrate

A lightweight Go library and CLI for running API-driven migrations defined in YAML files. It helps you automate provisioning or configuration tasks against HTTP APIs (e.g., create users/dashboards, import resources) in a versioned, repeatable way.

- Library: import and run migrations programmatically.
- CLI: run versioned migrations from a directory and record versions in a local SQLite store.
- Auth: built-in providers (oauth2, basic, pocketbase) and pluggable registry for custom providers.

## Features

- Versioned up/down migrations with persisted history (SQLite, `apimigrate.db`).
- Request templating with simple Go templates using layered environment variables.
- Config templating (Go templates {{.var}}) supported across requests, auth config, and wait checks.
- Response validation via allowed HTTP status codes.
- Response JSON extraction using `tidwall/gjson` paths.
- Optional "find" step for down migrations to discover IDs before deletion.
- Health-check wait feature to poll an endpoint until it returns the expected status before running migrations.
- HTTP client TLS options per config document (insecure, min/max TLS version).
- Pluggable auth provider registry with helper APIs and typed wrappers for library users.

## Install

- Library:

```bash
go get github.com/loykin/apimigrate
```

- CLI (from source):

```bash
go build -o apimigrate ./cmd/apimigrate
```

You can also run the CLI without building:

```bash
go run ./cmd/apimigrate
```

## Quick start (CLI)

Run immediately with the built-in example config and migration directory:

```bash
# from the repo root
go run ./cmd/apimigrate
```

What this does by default:
- Loads config from ./config/config.yaml
- Runs versioned migrations found under ./config/migration
- Records migration history in ./config/migration/apimigrate.db

There is a sample migration at config/migration/001_sample.yaml that calls https://example.com and should succeed out of the box.

Other useful commands:

```bash
# Apply up to a specific version (0 = all)
go run ./cmd/apimigrate up --to 0

# Roll back down to a target version (e.g., 0 to roll back all)
go run ./cmd/apimigrate down --to 0

# Show current and applied versions
go run ./cmd/apimigrate status
```

Customize:
- Use --config to point to a different YAML file (it must include migrate_dir):

```bash
go run ./cmd/apimigrate --config examples/keycloak_migration/config.yaml -v
```

Config YAML supports:
- auth: acquire and store tokens via providers, injected by logical name in tasks (request.auth_name or down.auth). String fields support Go templates ({{.var}}) rendered against env.
- migrate_dir: path to migrations (001_*.yaml, 002_*.yaml, ...).
- env: global key/value variables used in templating. You can also pull from OS env with valueFromEnv.
- wait: optional HTTP health check before running migrations (url/method/status/timeout/interval). The url supports templating (e.g., "{{.api_base}}/health").
- client: HTTP client TLS options (insecure, min_tls_version, max_tls_version) applied to requests and wait checks.
- store.save_response_body: when true, also stores response bodies in migration history.

See also: `examples/keycloak_migration/config.yaml` and `examples/grafana_migration/config.yaml`

## Migration file format

Each migration file contains an `up` and optionally a `down` section. Requests support templated fields; responses can validate status and extract values via gjson paths.

Example snippet:

```yaml
up:
  name: create user
  env:
    username: demo
  request:
    auth_name: keycloak
    method: POST
    url: "{{.kc_base}}/admin/realms/{{.realm}}/users"
    headers: []
    queries: []
    body: |
      {"username": "{{.username}}", "enabled": true}
  response:
    result_code: ["201", "409"]

down:
  name: delete user
  auth: keycloak
  find:
    request:
      method: GET
      url: "{{.kc_base}}/admin/realms/{{.realm}}/users?username={{.username}}&exact=true"
    response:
      result_code: ["200"]
      env_from:
        user_id: "0.id"   # gjson path into array element
  method: DELETE
  url: "{{.kc_base}}/admin/realms/{{.realm}}/users/{{.user_id}}"
```

Notes:
- Empty `result_code` means any HTTP status is allowed.
- `env_from` uses gjson paths (e.g., `id`, `0.id`, `data.items.0.id`).
- All values extracted via `env_from` are automatically persisted into the local store so they can be reused later (e.g., in down).

### Templating in config (requests, auth, wait)
- Only basic Go templates are supported: use `{{.var}}`.
- Templating applies in many string fields: request URL/headers/body, auth configs under `auth[].config`, and the `wait.url`.
- Variables come from layered env (global from config + local from each migration).
- YAML tip: when a field contains `{{...}}`, quote the string to avoid YAML parser confusion.

### Wait for service (health checks)
Before running migrations, you can wait until a service is healthy:

```yaml
wait:
  url: "{{.api_base}}/health"
  method: GET        # default: GET (also supports HEAD)
  status: 200        # default: 200
  timeout: 60s       # total time to wait (default 60s)
  interval: 2s       # polling interval (default 2s)

# Optional: TLS options used by wait and all HTTP requests
client:
  # insecure: false
  # min_tls_version: "1.2"
  # max_tls_version: "1.3"
```

## Programmatic usage (library)

```go
package main

import (
  "context"
  "github.com/loykin/apimigrate"
)

func main() {
  ctx := context.Background()
  base := apimigrate.Env{Global: map[string]string{"kc_base": "http://localhost:8080"}}
  // Apply all migrations in directory
  results, err := apimigrate.MigrateUp(ctx, "examples/keycloak_migration/migration", base, 0)
  if err != nil { panic(err) }
  _ = results
}
```

## Authentication providers

Built-in providers:
- oauth2 (multiple grants via internal/auth/oauth2)
- basic
- pocketbase

They can be configured via `config.yaml` (see examples) or acquired programmatically using the public API.

### Typed auth wrappers (library)
For convenient, type-safe auth acquisition without raw maps, use the wrappers in the root package:

```go
ctx := context.Background()
// Basic
h, v, name, err := apimigrate.AcquireBasicAuth(ctx, apimigrate.BasicAuthConfig{
  Name: "example_basic", Username: "admin", Password: "admin",
})
_ = h; _ = v; _ = name; _ = err

// OAuth2 Password
_, _, _, _ = apimigrate.AcquireOAuth2Password(ctx, apimigrate.OAuth2PasswordConfig{
  Name: "keycloak", ClientID: "admin-cli", TokenURL: "http://localhost:8080/realms/master/protocol/openid-connect/token",
  Username: "admin", Password: "root",
})
```
See `examples/auth_embedded` for a runnable sample.

### Registering a custom provider (for library users)

Use the re-exported API from the root package.

- Register your factory:

```go
apimigrate.RegisterAuthProvider("demo", func(spec map[string]interface{}) (apimigrate.AuthMethod, error) { /* ... */ })
```

- Acquire and store token by provider spec:

```go
h, v, name, err := apimigrate.AcquireAuthByProviderSpec(ctx, "demo", map[string]interface{}{"header": "X-Demo", "value": "ok"})
```

A complete runnable example is provided:
- `examples/auth_registry`

Run it:

```bash
go run ./examples/auth_registry
```

## Examples

- `examples/keycloak_migration`: Keycloak realm/user provisioning.
- `examples/grafana_migration`: Dashboard/user import for Grafana.
- `examples/embedded`: Minimal example running a single migration inline.
- `examples/auth_registry`: Demonstrates custom auth provider registration.
- `examples/auth_embedded`: Embed apimigrate and acquire auth via typed wrappers; uses a local test server.

Each example directory contains its own README or config and migration files.

## Development

- Run tests:

```bash
go test ./...
```

## License

This project is licensed under the MIT License. See LICENSE for details.
