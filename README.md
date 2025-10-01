# apirun

[![Coverage](https://img.shields.io/endpoint?url=https://raw.githubusercontent.com/loykin/apirun/gh-pages/shields/coverage.json&cacheSeconds=60)](https://github.com/loykin/apirun/blob/gh-pages/shields/coverage.json)
[![Go Report Card](https://goreportcard.com/badge/github.com/loykin/apirun)](https://goreportcard.com/report/github.com/loykin/apirun)
[![OpenSSF Scorecard](https://api.securityscorecards.dev/projects/github.com/loykin/apirun/badge)](https://securityscorecards.dev/viewer/?uri=github.com/loykin/apirun)
![CodeQL](https://github.com/loykin/apirun/actions/workflows/codeql.yml/badge.svg)
[![Trivy](https://img.shields.io/endpoint?url=https://raw.githubusercontent.com/loykin/apirun/gh-pages/shields/trivy.json&cacheSeconds=60)](https://raw.githubusercontent.com/loykin/apirun/gh-pages/shields/trivy.json)

A lightweight Go library and CLI for running API-driven automation workflows defined in YAML files. It helps you automate
provisioning, configuration tasks, and E2E testing against HTTP APIs (e.g., create users/dashboards, import resources) in a versioned,
repeatable way.

- **Library**: import and run API workflows programmatically.
- **CLI**: run versioned API workflows from a directory and record versions in a local SQLite store.
- **Multi-Stage Orchestration**: manage complex workflows with dependency management and environment variable propagation.
- **Auth**: built-in providers (oauth2, basic, pocketbase) and pluggable registry for custom providers.
- **Logging**: structured logging with slog, configurable levels and formats (text/JSON/color).
- **Security**: automatic masking of sensitive information in logs (passwords, tokens, secrets).
- **Enhanced monitoring**: workflow progress tracking with timing and HTTP request monitoring.

## Features

> **Recent changes (2025-10)**
> - **🚀 Multi-Stage Orchestration**: New `stages` command for managing complex workflows with multiple dependent stages
> - **🧹 CLI Simplification**: Removed redundant `--no-store` and `--dry-run-from` flags - use config file settings instead
> - **📁 Stage-based Architecture**: Each stage has its own config, migrations, and store while sharing environment variables
> - **🔗 Dependency Management**: Automatic execution ordering based on stage dependencies with cycle detection
> - **⚡ Partial Execution**: Run specific stages with `--from`, `--to`, or `--stage` flags
> - **🔍 Enhanced Dry-run**: Better execution planning and validation for both single migrations and multi-stage workflows

> **Previous changes (2025-09)**
> - New struct-based authentication API: type Auth { Type, Name, Methods } with method Acquire(ctx, env *env.Env)
> - Migrator now supports multiple auth entries: Migrator.Auth []auth.Auth. It auto-acquires tokens once at the start of MigrateUp/Down and injects them into templates under {{.auth.<name>}}
> - Template variables are grouped: use {{.env.key}} for your variables and {{.auth.name}} for acquired tokens
> - YAML headers must be a list of name/value objects (not a map). Example: headers: [ { name: Authorization, value: "Basic {{.auth.basic}}" } ]
> - Added examples demonstrating both embedded multi-auth and decoupled flows:
>   - examples/auth_embedded: single embedded auth
>   - examples/auth_embedded_multi_registry: multiple embedded auths
>   - examples/auth_embedded_multi_registry_type2: decoupled (acquire first, then migrate)
>   - examples/auth_embedded_lazy: multiple embedded auths acquired lazily on first use per auth

- Versioned up/down workflows with persisted history (SQLite, `apirun.db`).
- Request templating with simple Go templates using layered environment variables.
- DriverConfig templating (Go templates {{.var}}) supported across requests, auth config, and wait checks.
- Response validation via allowed HTTP status codes.
- Response JSON extraction using `tidwall/gjson` paths.
- Configurable handling when extracted variables are missing (env_missing: skip | fail).
- Optional "find" step for down migrations to discover IDs before deletion.
- Health-check wait feature to poll an endpoint until it returns the expected status before running migrations.
- HTTP client TLS options per config document (insecure, min/max TLS version). Default minimum TLS version is 1.3 unless
  overridden.
- Pluggable auth provider registry with helper APIs and typed wrappers for library users.
- Explicit header handling: providers return only token values; the library never auto-prefixes Authorization. Set
  headers like `Authorization: "Basic {{._auth_token}}"` or `Authorization: "Bearer {{._auth_token}}"` in your
  migrations.

## Install

- Library:

```bash
go get github.com/loykin/apirun
```

- CLI (from source):

```bash
go build -o apirun ./cmd/apirun
```

You can also run the CLI without building:

```bash
go run ./cmd/apirun
```

## Quick start (CLI)

Run immediately with the built-in example config and migration directory:

```bash
# from the repo root
go run ./cmd/apirun
```

What this does by default:

- Loads config from ./config/config.yaml
- Runs versioned migrations found under ./config/migration
- Records workflow history in ./config/migration/apirun.db

There is a sample migration at config/migration/001_sample.yaml that calls https://example.com and should succeed out of
the box.

Other useful commands:

```bash
# Create a new migration file with a timestamped prefix under migrate_dir
# Example output: ./config/migration/20250914004300_create_user.yaml
go run ./cmd/apirun create "create user"

# Apply up to a specific version (0 = all)
go run ./cmd/apirun up --to 0

# Roll back down to a target version (e.g., 0 to roll back all)
go run ./cmd/apirun down --to 0

# Show current and applied versions
go run ./cmd/apirun status

# Dry-run mode (planning without execution)
go run ./cmd/apirun up --dry-run
```

Customize:

- Use --config to point to a different YAML file (it must include migrate_dir):

```bash
go run ./cmd/apirun --config examples/keycloak_migration/config.yaml -v
```

DriverConfig YAML supports:

- auth: acquire and store tokens via providers, injected by logical name in tasks (request.auth_name or down.auth).
  String fields support Go templates ({{.var}}) rendered against env.
- migrate_dir: path to migrations (001_*.yaml, 002_*.yaml, ...).
- env: global key/value variables used in templating. You can also pull from OS env with valueFromEnv.
- wait: optional HTTP health check before running migrations (url/method/status/timeout/interval). The url supports
  templating (e.g., "{{.api_base}}/health").
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
  # Disable store operations entirely (stateless mode - no CLI flag equivalent)
  # When disabled, migrations run without persisting any state to database
  # disabled: false

  # Whether to record response bodies alongside status codes in migration history
  save_response_body: false

  # Backend type: "sqlite" (default) or "postgres"
  # type: sqlite

  # SQLite options (used when type is sqlite)
  sqlite:
  # path: ./config/migration/apirun.db   # default: <migrate_dir>/apirun.db

  # PostgreSQL options (used when type is postgres)
  # postgres:
  #   # Option A: provide a full DSN
  #   # dsn: postgres://user:pass@localhost:5432/apirun?sslmode=disable
  #
  #   # Option B: or provide components to build the DSN
  #   host: localhost
  #   port: 5432
  #   user: postgres
  #   password: postgres
  #   dbname: apirun
  #   sslmode: disable

# HTTP client TLS settings (optional, default minimum TLS is 1.3)
client:
# insecure: false
# min_tls_version: "1.2"   # or "tls1.2"
# max_tls_version: "1.3"   # or "tls1.3"

# Structured logging configuration (optional)
logging:
  level: info     # error, warn, info, debug
  format: text    # text, json, color (color auto-detects terminal capability)
  color: true     # enable/disable colors for format=color (optional, auto-detected if not set)

  # Sensitive data masking (optional, enabled by default)
  masking:
    enabled: true  # enable/disable masking of sensitive information
    # Custom patterns can be added here (see masking section below)
```

Notes:

- **Store Management**: All store settings are now exclusively managed via config files. The `--no-store` CLI flag has been removed for better consistency.
- If store.type is omitted or set to sqlite, apirun stores workflow history in a local SQLite file under <migrate_dir>/apirun.db by default. You can override the path via store.sqlite.path.
- To use PostgreSQL, set store.type to postgres and either:
    - provide store.postgres.dsn directly, or
    - provide the component fields (host/port/user/password/dbname[/sslmode]) and a DSN will be constructed.
- **Stateless Mode**: Set `store.disabled: true` to run migrations without persisting any state (replaces `--no-store` flag).
- The migration history schema is initialized automatically by the library (no external migration tool needed).
- Advanced: you can customize table names via store.table_prefix (derives three names automatically) or by setting store.table_schema_migrations, store.table_migration_runs, and store.table_stored_env individually (explicit names take precedence over the prefix).
- You can inspect current/applied versions with: `apirun status --config <path>`.

### Body templating default

By default, request bodies are rendered as Go templates when they contain {{...}}. You can disable this by:

- Setting request.render_body: false in a specific YAML request; or
- Setting the programmatic Migrator.RenderBodyDefault = false to make unannotated requests send the raw body as-is.

This is useful when you want to post JSON that legitimately includes template-like braces.

## Stateless Mode (No Store)

apirun can run migrations without persisting any state to a database. This is useful for:

- **CI/CD pipelines**: Run API setups without requiring database storage
- **Testing environments**: Execute workflows without leaving state behind
- **Ephemeral deployments**: Apply configurations without persistent storage
- **Development**: Quickly test migrations without affecting stored state

### Usage

**Configuration File:**
```yaml
store:
  disabled: true  # Disable store operations entirely
```

**Environment Variable:**
```bash
export APIRUN_NO_STORE=true
apirun up
```

> **Note**: The `--no-store` CLI flag has been removed in favor of config-file-only management for better consistency and maintainability.

### Behavior in Stateless Mode

When store operations are disabled:

- ✅ **Migrations execute normally**: All HTTP requests and API calls work as expected
- ✅ **Authentication works**: Auth providers acquire tokens normally
- ✅ **Environment variables work**: All templating and variable extraction functions
- ✅ **Response validation works**: Status codes and response parsing work normally
- ❌ **No version tracking**: Migration versions are not persisted or checked
- ❌ **No execution history**: No record of what was run or when
- ❌ **No stored environment**: Variables extracted via `env_from` are not persisted
- ❌ **Status command**: Shows "Store is disabled" instead of version info

### Important Notes

- **Configuration-driven**: Store behavior is now exclusively controlled via config files (`store.disabled: true`)
- **No rollback capability**: Without stored state, down migrations cannot reference previously extracted IDs
- **Idempotent workflows recommended**: Design migrations to handle repeated execution gracefully
- **Status command behavior**: Returns informational message instead of version data

This mode is particularly useful for setup scripts, testing scenarios, and environments where persistent state is not desired or available.

## Multi-Stage Orchestration

apirun supports complex workflows through multi-stage orchestration, allowing you to manage dependent stages with automatic execution ordering and environment variable propagation.

### Key Features

- **🔗 Dependency Management**: Define stage dependencies with automatic topological sorting
- **📁 Stage Isolation**: Each stage has its own config, migrations, and store
- **⚡ Environment Propagation**: Share variables between stages automatically
- **🎯 Partial Execution**: Run specific stages or ranges with `--from`, `--to`, `--stage`
- **🔍 Enhanced Planning**: Dry-run mode shows execution plan and dependencies
- **⚠️ Failure Handling**: Configurable behavior on stage failures
- **🔄 Dynamic Updates**: Add/remove migration files and stages safely during development

### Quick Start

```bash
# Create stages configuration
cat > stages.yaml << 'EOF'
apiVersion: apirun/v1
kind: StageOrchestration

stages:
  - name: infrastructure
    config_path: infra/config.yaml
    env:
      region: us-west-2

  - name: services
    config_path: services/config.yaml
    depends_on: [infrastructure]
    env_from_stages:
      - stage: infrastructure
        vars: [vpc_id, db_endpoint]

  - name: configuration
    config_path: config/config.yaml
    depends_on: [services]

global:
  env:
    project_name: my-project
  wait_between_stages: 10s
EOF

# Execute all stages
apirun stages up --config stages.yaml

# Execute specific stages
apirun stages up --from services --to configuration

# Show execution plan
apirun stages up --dry-run

# Check status
apirun stages status --verbose
```

### Stage Configuration

Each stage references its own config file with standard apirun configuration:

```yaml
# infra/config.yaml
migrate_dir: ./migration
env:
  - name: vpc_cidr
    value: 10.0.0.0/16
auth:
  - type: basic
    name: aws_api
    config:
      username: "{{.aws_key}}"
      password: "{{.aws_secret}}"
store:
  type: sqlite
  sqlite:
    path: ./infra_state.db
```

### Environment Variable Flow

```yaml
# Infrastructure stage extracts variables
response:
  env_from:
    vpc_id: "vpc.id"           # Extracted from API response
    db_endpoint: "db.endpoint"

# Services stage receives these variables
env_from_stages:
  - stage: infrastructure
    vars: [vpc_id, db_endpoint]  # Available as {{.vpc_id}}, {{.db_endpoint}}
```

### Advanced Features

#### Conditional Execution
```yaml
stages:
  - name: production-only
    condition: "{{.environment}} == 'production'"
    config_path: prod/config.yaml
```

#### Failure Handling
```yaml
stages:
  - name: optional-stage
    on_failure: continue        # Options: stop, continue, skip_dependents
    timeout: 300s
```

#### Global Settings
```yaml
global:
  env:
    project_name: my-project
    log_level: info
  wait_between_stages: 30s
  rollback_on_failure: true
  max_concurrent_stages: 3
```

### Available Commands

```bash
# Execute stages
apirun stages up [--from STAGE] [--to STAGE] [--stage STAGE] [--dry-run]
apirun stages down [--from STAGE] [--to STAGE] [--stage STAGE]

# Status and validation
apirun stages status [--verbose]
apirun stages validate
```

### Dependency Management & Constraints

#### 🔗 Dependency Rules
- **Acyclic Dependencies**: Circular dependencies are automatically detected and rejected
- **Execution Order**: Stages execute in topological order based on dependency graph
- **Isolated Execution**: Single stage execution requires all dependencies to be pre-executed
- **Environment Inheritance**: Dependent stages can inherit environment variables from parent stages

#### 📋 Constraints & Limitations
- **Template Variables**: Migrations using `{{.variable}}` templates require parent stages to export those variables
- **Database Isolation**: Each stage maintains its own migration state (separate store configurations recommended)
- **Partial Execution**: `--stage` flag only works if all dependencies have been previously executed
- **Variable Propagation**: Environment variables flow one-way (parent → child) in dependency chain

#### 🔄 Dynamic Changes During Development

##### ✅ Safe Operations
- **Add Migration Files**: New migration files in existing stages are automatically detected and executed
- **Remove Migration Files**: Deleted files don't affect already-applied migrations (state-based)
- **Add New Stages**: New stages can be safely added at any position in dependency chain
- **Modify Stage Configuration**: Update `config.yaml` files without affecting migration state

##### ⚠️ Operations Requiring Caution
- **Change Dependencies**: Modifying `depends_on` relationships may require state cleanup
- **Rename Stages**: Stage renaming breaks environment variable inheritance from previous runs
- **Template Dependencies**: Adding template variables to migrations requires ensuring parent stages export them

#### 🚨 Troubleshooting Common Issues

```bash
# Issue: "dependent stage X has not been executed"
# Solution: Run dependencies first or use --from flag
apirun stages up --from parent-stage --to target-stage

# Issue: "variable not found in dependent stage"
# Solution: Check if parent stage's migration exports the required variable
apirun stages status --verbose  # Check extracted variables

# Issue: Template rendering errors in migrations
# Solution: Verify environment variable propagation
apirun stages validate  # Validate configuration
```

### 📚 Additional Resources

- **[Migration Lifecycle Guide](MIGRATION_LIFECYCLE.md)**: Deep dive into migration file behavior in multi-stage contexts
- **[Troubleshooting Guide](TROUBLESHOOTING_STAGES.md)**: Common issues and solutions for multi-stage orchestration
- **[Complete Example](examples/stages/)**: Infrastructure → services → configuration workflow demonstration

## Structured Logging and Security

apirun provides comprehensive structured logging with security features:

### Logging Formats

Three logging formats are supported:

1. **Text format** (default): Human-readable console output
2. **JSON format**: Structured JSON logs for machine processing
3. **Color format**: Enhanced text format with ANSI color codes for CLI readability

The color format automatically detects terminal capabilities and adjusts accordingly:

- Full colors in interactive terminals
- No colors when redirected to files or non-terminal outputs
- Configurable through the `logging.color` setting

### Sensitive Data Masking

apirun automatically masks sensitive information in logs to prevent credential exposure:

- **Password fields**: `password`, `passwd`, `pwd`
- **API keys**: `api_key`, `apikey`, `api-key`
- **Tokens**: `token`, `access_token`, `auth_token`
- **Secrets**: `secret`, `client_secret`, `client-secret`
- **Authorization headers**: `Bearer` and `Basic` tokens
- **Custom patterns**: Support for user-defined sensitive patterns

Example of masked output:

```
2025-09-18T10:30:45Z [INFO ] Authenticating user username="admin" password="***MASKED***" api_key="***MASKED***"
```

### Enhanced Progress Monitoring

The logging system includes detailed migration progress tracking:

- **Step-by-step progress**: Each migration step with timing information
- **HTTP request monitoring**: Method, URL, status code, and response time
- **Success/failure indicators**: Clear visual feedback with appropriate colors
- **Performance metrics**: Request latency and total execution time

Example progress output:

```
2025-09-18T10:30:45Z [INFO ] Starting migration step=1 name="Create user account"
2025-09-18T10:30:45Z [INFO ] HTTP request method=POST url="https://api.example.com/users" 
2025-09-18T10:30:46Z [INFO ] HTTP response status=201 duration=890ms success=true
2025-09-18T10:30:46Z [INFO ] Migration completed step=1 success=true duration=1.2s
```

### Programmatic Logging API

Use the logging API in your Go applications:

```go
package main

import (
	"github.com/loykin/apirun"
)

func main() {
	// Create different logger types
	textLogger := apirun.NewLogger(apirun.LogLevelInfo)
	jsonLogger := apirun.NewJSONLogger(apirun.LogLevelInfo)
	colorLogger := apirun.NewColorLogger(apirun.LogLevelInfo)

	// Set as default logger
	apirun.SetDefaultLogger(colorLogger)

	// Configure masking
	masker := apirun.NewMasker()
	apirun.SetGlobalMasker(masker)
	apirun.EnableMasking(true)

	logger := apirun.GetLogger()
	logger.Info("Starting migration", "version", 1, "user", "admin")
}
```

### Masking API

Control sensitive data masking programmatically:

```go
// Create custom masker with patterns
patterns := []apirun.SensitivePattern{
{
Name: "custom_secret",
Keys: []string{"custom_key", "secret_data"},
},
}
masker := apirun.NewMaskerWithPatterns(patterns)

// Mask sensitive data directly
cleaned := apirun.MaskSensitiveData(`{"password": "secret123", "username": "admin"}`)
// Result: {"password": "***MASKED***", "username": "admin"}

// Control global masking
apirun.EnableMasking(false) // Disable masking
enabled := apirun.IsMaskingEnabled() // Check status
```

### Customizing store table names (rules)

You can change the SQLite/PostgreSQL table (and index) names used to persist the migration history. There are two ways
to configure names under the store section:

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
- If a provided name does not match the allowed pattern, apirun falls back to the safe default for that identifier.
- Do not include quotes, dots, or schema qualifiers in names; use plain identifiers (e.g., my_schema_migrations, not
  public.my_schema_migrations). The default schema of your database connection will be used.
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

Each migration file contains an `up` and optionally a `down` section. Requests support templated fields; responses can
validate status and extract values via gjson paths.

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
    headers: [ ]
    queries: [ ]
    body: |
      {"username": "{{.username}}", "enabled": true}
  response:
    result_code: [ "201", "409" ]

down:
  name: delete user
  auth: keycloak
  find:
    request:
      method: GET
      url: "{{.kc_base}}/admin/realms/{{.realm}}/users?username={{.username}}&exact=true"
    response:
      result_code: [ "200" ]
      env_from:
        user_id: "0.id"   # gjson path into array element
  method: DELETE
  url: "{{.kc_base}}/admin/realms/{{.realm}}/users/{{.user_id}}"
```

Notes:

- Empty `result_code` means any HTTP status is allowed.
- `env_from` uses gjson paths (e.g., `id`, `0.id`, `data.items.0.id`).
- All values extracted via `env_from` are automatically persisted into the local store so they can be reused later (
  e.g., in down).
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
- Authorization headers are not auto-prefixed. When using a token acquired via `auth_name` or injected `_auth_token`,
  set the header explicitly in your migration, e.g., `Authorization: "Basic {{._auth_token}}"` for Basic or
  `Authorization: "Bearer {{._auth_token}}"` for OAuth2.

### Templating in config (requests, auth, wait)

- Only basic Go templates are supported: use `{{.var}}`.
- Templating applies in many string fields: request URL/headers/body, auth configs under `auth[].config`, and the
  `wait.url`.
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
"github.com/loykin/apirun"
)

func main() {
ctx := context.Background()
base := apirun.Env{Global: map[string]string{"kc_base": "http://localhost:8080"}}
// Apply all migrations in directory
m := apirun.Migrator{Dir: "examples/keycloak_migration/migration", Env: base}
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

Use the struct-based API to configure providers and acquire tokens. Tokens are values; set headers explicitly in
migrations.

- Embedded (automatic) acquisition with multiple providers:

```go
ctx := context.Background()
base := apirun.Env{Global: map[string]string{"api_base": srvURL}}

// Configure two Basic providers under names a1 and a2
auth1 := apirun.Auth{Type: apirun.AuthTypeBasic, Name: "a1", Methods: apirun.BasicAuthConfig{Username: "u1", Password: "p1"}}
auth2 := apirun.Auth{Type: apirun.AuthTypeBasic, Name: "a2", Methods: apirun.BasicAuthConfig{Username: "u2", Password: "p2"}}

m := apirun.Migrator{Dir: "./migs", Env: base, StoreConfig: &apirun.StoreConfig{}}
m.Auth = []apirun.Auth{auth1, auth2}
_, err := m.MigrateUp(ctx, 0)
```

- Decoupled acquisition (acquire first, then migrate):

```go
ctx := context.Background()
base := apirun.Env{Global: map[string]string{"api_base": srvURL}}

a := &apirun.Auth{Type: apirun.AuthTypeBasic, Name: "basic", Methods: apirun.BasicAuthConfig{Username: "admin", Password: "admin"}}
if v, err := a.Acquire(ctx, &base); err == nil {
if base.Auth == nil { base.Auth = map[string]string{} }
base.Auth["basic"] = v
}

m := apirun.Migrator{Dir: "./migs", Env: base, StoreConfig: &apirun.StoreConfig{}}
_, err := m.MigrateUp(ctx, 0)
```

See examples:

- examples/auth_embedded
- examples/auth_embedded_multi_registry
- examples/auth_embedded_multi_registry_type2
- examples/auth_embedded_lazy

### Registering a custom provider (for library users)

Use the re-exported API from the root package.

- Register your factory:

```go
apirun.RegisterAuthProvider("demo", func (spec map[string]interface{}) (apirun.AuthMethod, error) { /* ... */ })
```

- Acquire and store token by provider spec (store under a logical name):

```go
v, err := apirun.AcquireAuthByProviderSpecWithName(ctx, "demo", "my-demo", map[string]interface{}{"value": "ok"})
_ = v; _ = err
```

A complete runnable example is provided:

- `examples/auth_registry`

Run it:

```bash
go run ./examples/auth_registry
```

## Examples

### Single Migration Examples
- `examples/keycloak_migration`: Keycloak realm/user provisioning.
- `examples/grafana_migration`: Dashboard/user import for Grafana.
- `examples/embedded`: Minimal example running a single migration inline.
- `examples/embedded_sqlite`: Programmatic example using the default SQLite store with versioned migrations.
- `examples/embedded_postgresql`: Programmatic example using a PostgreSQL store with versioned migrations.
- `examples/embedded_custom_table`: Programmatic example demonstrating custom table/index names for the store.

### Multi-Stage Orchestration Examples
- **`examples/stages`**: Complete multi-stage workflow demonstrating infrastructure → services → configuration deployment with dependency management and environment variable propagation.

### Authentication Examples
- `examples/auth_registry`: Demonstrates custom auth provider registration.
- `examples/auth_embedded`: Embed apirun and acquire auth via typed wrappers; uses a local test server.

### Utility Examples
- `examples/color_demo`: Demonstrates different logging formats (text/JSON/color) for comparison.

Each example directory contains its own README or config and migration files.

### Color Logging Demo

To see the different logging formats in action:

```bash
# Run with text format (default)
go run ./examples/color_demo

# Run with JSON format  
go run ./examples/color_demo --format json

# Run with color format
go run ./examples/color_demo --format color

# Run with colors disabled
go run ./examples/color_demo --format color --no-color
```

The color demo shows:

- Different log levels with appropriate colors
- Sensitive data masking in action
- HTTP request/response logging
- Migration progress tracking
- Error and success highlighting

## Run history and status (CLI and library)

- Each Up/Down execution is recorded in the migration_runs table with:
    - version, direction (up|down), status_code, ran_at (RFC3339), and optionally the response body (when configured).
    - env_json: JSON of variables extracted by env_from.
    - failed: boolean indicating whether the operation returned an error (e.g., disallowed status or env_missing=fail).
- You can query this table directly (SQLite or Postgres) for auditing or troubleshooting.
- New: A high-level status command and helper allow you to see the current version, the list of applied versions, and
  optionally the execution history.

### CLI: status

- Show current and applied versions only:

```bash
apirun status --config <path/to/config.yaml>
```

Example output (no history):

```
current: 1
applied: [1]
```

- Include run history as well:

```bash
apirun status --config <path/to/config.yaml> --history
```

Example output (with history):

```
current: 2
applied: [1 2]
history:
#2 v=2 dir=up code=200 failed=false at=2025-09-13T12:01:00Z
#1 v=1 dir=up code=200 failed=false at=2025-09-13T12:00:00Z
```

Notes:

- By default, when you pass --history the CLI shows up to 10 latest entries in newest-first order.
- Use --history-all to print all entries (still newest-first), or --history-limit N to show a specific number.
- The --config file can specify migrate_dir and store settings; when omitted, the CLI defaults to ./config/migration and
  a local SQLite store at ./config/migration/apirun.db.
- The history lines include the run id, version, direction, HTTP status code, whether it failed, and the timestamp.

### Library: pkg/status helper

You can obtain the same information from your Go code using the pkg/status helper:

```go
info, err := status.FromOptions("./config/migration", nil) // nil => default SQLite at <dir>/apirun.db
if err != nil { /* handle */ }
fmt.Print(info.FormatHuman(false)) // or true to include history
```

- FormatHuman(false) prints just current/applied; FormatHuman(true) appends a formatted history section.
- If you already have an opened store via apirun.OpenStoreFromOptions, use status.FromStore(store).

### Example: examples/status_embedded

- A runnable example that first applies pending migrations and then prints the status, so the history section shows real
  entries.
- Run it from the repo root:

```bash
go run ./examples/status_embedded            # no history
go run ./examples/status_embedded --history  # with history
```

The example defaults to the directory examples/status_embedded/migration and uses a local SQLite database file there.

## Development

- Run tests:

```bash
go test ./...
```

## License

This project is licensed under the MIT License. See LICENSE for details.
