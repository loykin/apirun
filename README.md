# apimigrate

[![Coverage](https://img.shields.io/endpoint?url=https://raw.githubusercontent.com/loykin/apimigrate/gh-pages/shields/coverage.json&cacheSeconds=60)](https://github.com/loykin/apimigrate/blob/gh-pages/shields/coverage.json)
[![Go Report Card](https://goreportcard.com/badge/github.com/loykin/apimigrate)](https://goreportcard.com/report/github.com/loykin/apimigrate)
[![OpenSSF Scorecard](https://api.securityscorecards.dev/projects/github.com/loykin/apimigrate/badge)](https://securityscorecards.dev/viewer/?uri=github.com/loykin/apimigrate)
![CodeQL](https://github.com/loykin/apimigrate/actions/workflows/codeql.yml/badge.svg)
[![Trivy](https://img.shields.io/endpoint?url=https://raw.githubusercontent.com/loykin/apimigrate/gh-pages/shields/trivy.json&cacheSeconds=60)](https://raw.githubusercontent.com/loykin/apimigrate/gh-pages/shields/trivy.json)

A lightweight Go library and CLI for running API-driven migrations defined in YAML files. It helps you automate provisioning or configuration tasks against HTTP APIs (e.g., create users/dashboards, import resources) in a versioned, repeatable way.

- Library: import and run migrations programmatically.
- CLI: run versioned migrations from a directory and record versions in a local SQLite store.
- Auth: built-in providers (oauth2, basic, pocketbase) and pluggable registry for custom providers.

## Features

> Recent changes (2025-09)
> - New struct-based authentication API: type Auth { Type, Name, Methods } with method Acquire(ctx, env *env.Env).
> - Migrator now supports multiple auth entries: Migrator.Auth []auth.Auth. It auto-acquires tokens once at the start of MigrateUp/Down and injects them into templates under {{.auth.<name>}}.
> - Legacy helpers removed from public API: AcquireAuthAndSetEnv, AcquireAuthByProviderSpecWithName, and AuthSpec. Use the Auth struct and MethodConfig instead (e.g., BasicAuthConfig, OAuth2* configs, or NewAuthSpecFromMap).
> - Template variables are grouped: use {{.env.key}} for your variables and {{.auth.name}} for acquired tokens.
> - YAML headers must be a list of name/value objects (not a map). Example: headers: [ { name: Authorization, value: "Basic {{.auth.basic}}" } ].
> - Added examples demonstrating both embedded multi-auth and decoupled flows:
>   - examples/auth_embedded: single embedded auth.
>   - examples/auth_embedded_multi_registry: multiple embedded auths.
>   - examples/auth_embedded_multi_registry_type2: decoupled (acquire first, then migrate).

- Versioned up/down migrations with persisted history (SQLite, `apimigrate.db`).
- Request templating with simple Go templates using layered environment variables.
- DriverConfig templating (Go templates {{.var}}) supported across requests, auth config, and wait checks.
- Response validation via allowed HTTP status codes.
- Response JSON extraction using `tidwall/gjson` paths.
- Configurable handling when extracted variables are missing (env_missing: skip | fail).
- Optional "find" step for down migrations to discover IDs before deletion.
- Health-check wait feature to poll an endpoint until it returns the expected status before running migrations.
- HTTP client TLS options per config document (insecure, min/max TLS version). Default minimum TLS version is 1.3 unless overridden.
- Pluggable auth provider registry with helper APIs and typed wrappers for library users.
- Explicit header handling: providers return only token values; the library never auto-prefixes Authorization. Set headers like `Authorization: "Basic {{._auth_token}}"` or `Authorization: "Bearer {{._auth_token}}"` in your migrations.

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

DriverConfig YAML supports:
- auth: acquire and store tokens via providers, injected by logical name in tasks (request.auth_name or down.auth). String fields support Go templates ({{.var}}) rendered against env.
- migrate_dir: path to migrations (001_*.yaml, 002_*.yaml, ...).
- env: global key/value variables used in templating. You can also pull from OS env with valueFromEnv.
- wait: optional HTTP health check before running migrations (url/method/status/timeout/interval). The url supports templating (e.g., "{{.api_base}}/health").
- client: HTTP client TLS options (insecure, min_tls_version, max_tls_version) applied to requests and wait checks.
- store: choose the persistence backend (sqlite or postgres) and whether to save response bodies.

### Configuration (config.yaml)

Below is a complete example with inline comments. You can copy this into your project and adjust values.

```yaml
---
# Define one or more auth providers. Each provider has a type, a top-level name,
# and a config.
auth:
  - type: basic
    name: example_basic          # referenced by request.auth_name in migrations
    config:
      username: admin
      password: admin

# Directory that contains your versioned migrations (001_*.yaml, 002_*.yaml, ...)
migrate_dir: ./config/migration

# Optional: wait for an HTTP health check to succeed before running migrations
wait:
  # url: "{{.api_base}}/health"
  # method: GET       # default: GET
  # status: 200       # default: 200
  # timeout: 30s      # default: 60s
  # interval: 1s      # default: 2s

# Global environment variables available to all migrations
env:
  - name: api_base
    value: http://localhost:3000
  - name: example_user
    value: sample
  # - name: from_os
  #   valueFromEnv: EXAMPLE_FROM_OS

# Store settings
store:
  # Whether to record response bodies alongside status codes in migration history
  save_response_body: false

  # Backend type: "sqlite" (default) or "postgres"
  # type: sqlite

  # SQLite options (used when type is sqlite)
  sqlite:
    # path: ./config/migration/apimigrate.db   # default: <migrate_dir>/apimigrate.db

  # PostgreSQL options (used when type is postgres)
  # postgres:
  #   # Option A: provide a full DSN
  #   # dsn: postgres://user:pass@localhost:5432/apimigrate?sslmode=disable
  #
  #   # Option B: or provide components to build the DSN
  #   host: localhost
  #   port: 5432
  #   user: postgres
  #   password: postgres
  #   dbname: apimigrate
  #   sslmode: disable

# HTTP client TLS settings (optional, default minimum TLS is 1.3)
client:
  # insecure: false
  # min_tls_version: "1.2"   # or "tls1.2"
  # max_tls_version: "1.3"   # or "tls1.3"
```

Notes:
- If store.type is omitted or set to sqlite, apimigrate stores migration history in a local SQLite file under <migrate_dir>/apimigrate.db by default. You can override the path via store.sqlite.path.
- To use PostgreSQL, set store.type to postgres and either:
  - provide store.postgres.dsn directly, or
  - provide the component fields (host/port/user/password/dbname[/sslmode]) and a DSN will be constructed.
- The migration history schema is initialized automatically by the library (no external migration tool needed).
- Advanced: you can customize table names via store.table_prefix (derives three names automatically) or by setting store.table_schema_migrations, store.table_migration_runs, and store.table_stored_env individually (explicit names take precedence over the prefix).
- You can inspect current/applied versions with: `apimigrate status --config <path>`.

### Customizing store table names (rules)
You can change the SQLite/PostgreSQL table (and index) names used to persist the migration history. There are two ways to configure names under the store section:

1) table_prefix (simple):
- Derives names automatically as:
  - <prefix>_schema_migrations
  - <prefix>_migration_runs
  - <prefix>_stored_env
- Example:
  store:
    table_prefix: app1
  results in: app1_schema_migrations, app1_migration_runs, app1_stored_env

2) Explicit names (fine-grained):
- Set one or more of these fields to override individually:
  - table_schema_migrations
  - table_migration_runs
  - table_stored_env
  - index_stored_env_by_version (optional index name, reserved for future use)
- When both prefix and explicit names are provided, explicit names take precedence for the fields you set.

Validation and constraints:
- Allowed identifier characters: only ASCII letters, digits and underscores, starting with a letter or underscore.
  - Regex: ^[a-zA-Z_][a-zA-Z0-9_]*$
- If a provided name does not match the allowed pattern, apimigrate falls back to the safe default for that identifier.
- Do not include quotes, dots, or schema qualifiers in names; use plain identifiers (e.g., my_schema_migrations, not public.my_schema_migrations). The default schema of your database connection will be used.
- The same rules apply to both SQLite and PostgreSQL backends.

Examples (YAML):
- Using a single prefix:
  ```yaml
  store:
    type: sqlite
    table_prefix: app1
  ```

- Overriding specific names:
  ```yaml
  store:
    type: postgres
    postgres:
      dsn: postgres://user:pass@localhost:5432/postgres?sslmode=disable
    table_schema_migrations: app_schema
    table_migration_runs: app_runs
    table_stored_env: app_env
  ```

Programmatic (library) equivalent:
- Construct a Migrator and set StoreConfig with driver-specific options and optional custom table names (pseudo-code):
  - Set Driver to postgres or sqlite
  - Provide DSN or sqlite path
  - Optionally customize table names

See also:
- `config/config.yaml` (commented template)
- `examples/keycloak_migration/config.yaml`
- `examples/grafana_migration/config.yaml`
- Embedded examples: `examples/embedded`, `examples/embedded_postgresql`

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
- Control behavior for missing extractions with `env_missing` under `response`:
  - `skip` (default): ignore missing keys in `env_from` and continue.
  - `fail`: treat missing keys as an error; the migration run will be recorded with failed=true.
  Example:
  ```yaml
  response:
    result_code: ["200"]
    env_missing: fail
    env_from:
      rid: id           # required; if absent -> error
      optional: maybe   # if missing, execution fails in 'fail' mode
  ```
- Authorization headers are not auto-prefixed. When using a token acquired via `auth_name` or injected `_auth_token`, set the header explicitly in your migration, e.g., `Authorization: "Basic {{._auth_token}}"` for Basic or `Authorization: "Bearer {{._auth_token}}"` for OAuth2.

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
# NOTE: This is illustrative code, not compiled within README.
// Example: using the struct-based Migrator API
package main

import (
  "context"
  "github.com/loykin/apimigrate"
)

func main() {
  ctx := context.Background()
  base := apimigrate.Env{Global: map[string]string{"kc_base": "http://localhost:8080"}}
  // Apply all migrations in directory
  m := apimigrate.Migrator{Dir: "examples/keycloak_migration/migration", Env: base}
    results, err := m.MigrateUp(ctx, 0)
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

### Struct-based Auth API (library)
Use the struct-based API to configure providers and acquire tokens. Tokens are values; set headers explicitly in migrations.

- Embedded (automatic) acquisition with multiple providers:

```go
ctx := context.Background()
base := apimigrate.Env{Global: map[string]string{"api_base": srvURL}}

// Configure two Basic providers under names a1 and a2
auth1 := apimigrate.Auth{Type: apimigrate.AuthTypeBasic, Name: "a1", Methods: map[string]apimigrate.MethodConfig{
  apimigrate.AuthTypeBasic: apimigrate.BasicAuthConfig{Username: "u1", Password: "p1"},
}}
auth2 := apimigrate.Auth{Type: apimigrate.AuthTypeBasic, Name: "a2", Methods: map[string]apimigrate.MethodConfig{
  apimigrate.AuthTypeBasic: apimigrate.BasicAuthConfig{Username: "u2", Password: "p2"},
}}

m := apimigrate.Migrator{Dir: "./migs", Env: base, StoreConfig: &apimigrate.StoreConfig{}}
m.Auth = []apimigrate.Auth{auth1, auth2}
_, err := m.MigrateUp(ctx, 0)
```

- Decoupled acquisition (acquire first, then migrate):

```go
ctx := context.Background()
base := apimigrate.Env{Global: map[string]string{"api_base": srvURL}}

a := &apimigrate.Auth{Type: apimigrate.AuthTypeBasic, Name: "basic", Methods: map[string]apimigrate.MethodConfig{
  apimigrate.AuthTypeBasic: apimigrate.BasicAuthConfig{Username: "admin", Password: "admin"},
}}
if v, err := a.Acquire(ctx, &base); err == nil {
  if base.Auth == nil { base.Auth = map[string]string{} }
  base.Auth["basic"] = v
}

m := apimigrate.Migrator{Dir: "./migs", Env: base, StoreConfig: &apimigrate.StoreConfig{}}
_, err := m.MigrateUp(ctx, 0)
```

See examples:
- examples/auth_embedded
- examples/auth_embedded_multi_registry
- examples/auth_embedded_multi_registry_type2

### Registering a custom provider (for library users)

Use the re-exported API from the root package.

- Register your factory:

```go
apimigrate.RegisterAuthProvider("demo", func(spec map[string]interface{}) (apimigrate.AuthMethod, error) { /* ... */ })
```

- Acquire and store token by provider spec (store under a logical name):

```go
v, err := apimigrate.AcquireAuthByProviderSpecWithName(ctx, "demo", "my-demo", map[string]interface{}{"value": "ok"})
_ = v; _ = err
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
- `examples/embedded_sqlite`: Programmatic example using the default SQLite store with versioned migrations.
- `examples/embedded_postgresql`: Programmatic example using a PostgreSQL store with versioned migrations.
- `examples/embedded_custom_table`: Programmatic example demonstrating custom table/index names for the store.
- `examples/auth_registry`: Demonstrates custom auth provider registration.
- `examples/auth_embedded`: Embed apimigrate and acquire auth via typed wrappers; uses a local test server.

Each example directory contains its own README or config and migration files.

## Run history and failure flag

- Each Up/Down execution is recorded in the migration_runs table with:
  - version, direction (up|down), status_code, ran_at, and optionally the response body (when configured).
  - env_json: JSON of variables extracted by env_from.
  - failed: boolean indicating whether the operation returned an error (e.g., disallowed status or env_missing=fail).
- You can query this table directly (SQLite or Postgres) for auditing or troubleshooting. The CLI currently provides a high-level status command:

```bash
apimigrate status --config <path/to/config.yaml>
```

## Development

- Run tests:

```bash
go test ./...
```

## License

This project is licensed under the MIT License. See LICENSE for details.
