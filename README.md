# apirun

[![Coverage](https://img.shields.io/endpoint?url=https://raw.githubusercontent.com/loykin/apirun/gh-pages/shields/coverage.json&cacheSeconds=60)](https://github.com/loykin/apirun/blob/gh-pages/shields/coverage.json)
[![Go Report Card](https://goreportcard.com/badge/github.com/loykin/apirun)](https://goreportcard.com/report/github.com/loykin/apirun)
[![OpenSSF Scorecard](https://api.securityscorecards.dev/projects/github.com/loykin/apirun/badge)](https://securityscorecards.dev/viewer/?uri=github.com/loykin/apirun)
![CodeQL](https://github.com/loykin/apirun/actions/workflows/codeql.yml/badge.svg)
[![Trivy](https://img.shields.io/endpoint?url=https://raw.githubusercontent.com/loykin/apirun/gh-pages/shields/trivy.json&cacheSeconds=60)](https://raw.githubusercontent.com/loykin/apirun/gh-pages/shields/trivy.json)

A lightweight Go library and CLI for running API-driven automation workflows defined in YAML files.

**Key Features:**
- **Versioned Migrations**: Run HTTP API workflows with up/down migration support
- **Multi-Stage Orchestration**: Manage complex workflows with dependency management
- **Authentication**: Built-in providers (oauth2, basic, pocketbase) with custom registry support
- **Structured Logging**: Configurable logging with sensitive data masking
- **Templating**: Go template support for dynamic requests and configurations

## Features

- **Versioned Migrations**: Up/down workflows with SQLite/PostgreSQL history tracking
- **Multi-Stage Orchestration**: Dependent stages with automatic execution ordering
- **Request Templating**: Go templates with environment variables and auth tokens
- **Response Processing**: JSON extraction, status validation, variable extraction
- **Authentication**: OAuth2, Basic Auth, PocketBase, and custom providers
- **Flexible Storage**: SQLite (default) or PostgreSQL backends
- **Health Checks**: Wait for services before running migrations
- **TLS Support**: Configurable TLS options per client
- **Structured Logging**: Text/JSON/Color formats with sensitive data masking

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

## Configuration

Basic configuration example:

```yaml
auth:
  - type: basic
    name: example_basic
    config:
      username: admin
      password: "{{.admin_password}}"

migrate_dir: ./migrations
env:
  - name: api_base
    value: http://localhost:3000
  - name: admin_password
    valueFromEnv: ADMIN_PASSWORD

store:
  type: sqlite
logging:
  level: info
  format: text
```

📖 **[Complete Configuration Reference →](docs/configuration.md)**

### Migration File Format

```yaml
up:
  name: create user
  request:
    auth_name: example_basic
    method: POST
    url: "{{.api_base}}/users"
    body: |
      {"username": "{{.username}}", "enabled": true}
  response:
    result_code: ["201"]
    env_from:
      user_id: "id"

down:
  name: delete user
  auth: example_basic
  method: DELETE
  url: "{{.api_base}}/users/{{.user_id}}"
```

📖 **[Complete Migration Format Reference →](docs/migration-format.md)**

## Stateless Mode

Run migrations without persisting state by setting `store.disabled: true` in config. Useful for CI/CD pipelines and testing.

## Multi-Stage Orchestration

Manage complex workflows with dependent stages and automatic execution ordering.

```bash
# Execute all stages
apirun stages up --config stages.yaml

# Execute specific stages
apirun stages up --from infrastructure --to services

# Dry run and status
apirun stages up --dry-run
apirun stages status --verbose
```

📖 **[Complete Multi-Stage Guide →](docs/multi-stage.md)**

## Logging and Security

```yaml
logging:
  level: info    # error, warn, info, debug
  format: text   # text, json, color
  masking:
    enabled: true  # automatic masking of sensitive data
```

Automatic masking of passwords, tokens, API keys in logs.

## Authentication

Built-in providers: Basic Auth, OAuth2, PocketBase. Custom providers supported via registry.

```yaml
auth:
  - type: basic
    name: api_basic
    config:
      username: admin
      password: "{{.admin_password}}"
  - type: oauth2
    name: github_oauth
    config:
      client_id: "{{.github_client_id}}"
      client_secret: "{{.github_client_secret}}"
      token_url: "https://github.com/login/oauth/access_token"
```

📖 **[Complete Authentication Guide →](docs/authentication.md)**

## Examples

### Single Migration Examples
- `examples/keycloak_migration`: Keycloak realm/user provisioning
- `examples/grafana_migration`: Dashboard/user import for Grafana
- `examples/embedded`: Minimal programmatic example

### Multi-Stage Examples
- `examples/stages`: Complete infrastructure → services → configuration workflow
- `examples/orchestrator_embedded`: Programmatic multi-stage orchestration

### Authentication Examples
- `examples/auth_embedded`: Basic auth integration
- `examples/auth_registry`: Custom auth provider registration

## Development

```bash
# Run tests
go test ./...

# Build CLI
go build -o apirun ./cmd/apirun
```

## Status and History

```bash
# Check migration status
apirun status

# Include execution history
apirun status --history

# Multi-stage status
apirun stages status --verbose
```

## Documentation

- 📖 **[Configuration Reference](docs/configuration.md)** - Complete config.yaml reference
- 📖 **[Migration Format](docs/migration-format.md)** - Migration file format and templating
- 📖 **[Authentication Guide](docs/authentication.md)** - Auth providers and usage
- 📖 **[Multi-Stage Orchestration](docs/multi-stage.md)** - Complex workflow management

## License

This project is licensed under the MIT License. See LICENSE for details.
