# apimigrate

A lightweight Go library and CLI for running API-driven migrations defined in YAML files. It helps you automate provisioning or configuration tasks against HTTP APIs (e.g., create users/dashboards, import resources) in a versioned, repeatable way.

- Library: import and run migrations programmatically.
- CLI: run versioned migrations from a directory and record versions in a local SQLite store.
- Auth: built-in providers (oauth2, basic, pocketbase) and pluggable registry for custom providers.

## Features

- Versioned up/down migrations with persisted history (SQLite, `apimigrate.db`).
- Request templating with simple Go templates using layered environment variables.
- Response validation via allowed HTTP status codes.
- Response JSON extraction using `tidwall/gjson` paths.
- Optional "find" step for down migrations to discover IDs before deletion.
- Pluggable auth provider registry with helper APIs for library users.

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

- Run migrations in a directory (defaults to examples/migration if not set):

```bash
go run ./cmd/apimigrate --config examples/keycloak_migration/config.yaml -v
```

- See current status / migrate specific versions:

```bash
go run ./cmd/apimigrate up --to 0
go run ./cmd/apimigrate down --to 0
go run ./cmd/apimigrate status
```

Config example (YAML) supports:
- auth: acquire and store tokens via providers, then inject by logical name in tasks.
- migrate_dir: path to migrations.
- env: global key/value variables used in templating.

See: `examples/keycloak_migration/config.yaml` and `examples/grafana_migration/config.yaml`

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

Each example directory contains its own README or config and migration files.

## Development

- Run tests:

```bash
go test ./...
```

## License

This project is licensed under the MIT License. See LICENSE for details.
