# Configuration Reference

Complete reference for configuring apirun projects.

## Basic Configuration Structure

```yaml
# config.yaml
auth:           # Authentication providers
migrate_dir:    # Migration directory
env:           # Environment variables
store:         # Database storage settings
wait:          # Health check configuration
client:        # HTTP client settings
logging:       # Logging configuration
```

## Authentication Configuration

### Basic Authentication

```yaml
auth:
  - type: basic
    name: api_basic
    config:
      username: admin
      password: "{{.admin_password}}"  # supports templating
```

### OAuth2 Authentication

```yaml
auth:
  - type: oauth2
    name: github_oauth
    config:
      client_id: "{{.github_client_id}}"
      client_secret: "{{.github_client_secret}}"
      token_url: "https://github.com/login/oauth/access_token"
      grant_type: client_credentials  # or authorization_code
      scopes: ["read:user", "repo"]
```

### PocketBase Authentication

```yaml
auth:
  - type: pocketbase
    name: pb_auth
    config:
      base_url: "{{.pb_base_url}}"
      username: "{{.pb_username}}"
      password: "{{.pb_password}}"
```

### Multiple Auth Providers

```yaml
auth:
  - type: basic
    name: admin_auth
    config:
      username: admin
      password: "{{.admin_pass}}"

  - type: oauth2
    name: api_oauth
    config:
      client_id: "{{.oauth_client_id}}"
      client_secret: "{{.oauth_client_secret}}"
      token_url: "{{.oauth_token_url}}"
```

## Environment Variables

### Static Variables

```yaml
env:
  - name: api_base
    value: https://api.example.com
  - name: environment
    value: production
  - name: timeout
    value: "30s"
```

### Environment Variable References

```yaml
env:
  - name: api_token
    valueFromEnv: API_TOKEN  # reads from OS environment
  - name: db_password
    valueFromEnv: DATABASE_PASSWORD
  - name: user_home
    valueFromEnv: HOME
```

### Mixed Configuration

```yaml
env:
  - name: api_base
    value: https://api.example.com
  - name: api_token
    valueFromEnv: API_TOKEN
  - name: environment
    value: "{{.ENVIRONMENT | default \"development\"}}"
```

## Store Configuration

### SQLite Store (Default)

```yaml
store:
  type: sqlite
  save_response_body: false  # whether to store HTTP response bodies
  sqlite:
    path: ./migration/apirun.db  # default: <migrate_dir>/apirun.db
```

### PostgreSQL Store

```yaml
store:
  type: postgres
  save_response_body: true
  postgres:
    # Option 1: Full DSN
    dsn: postgres://user:pass@localhost:5432/apirun?sslmode=disable

    # Option 2: Component-based
    # host: localhost
    # port: 5432
    # user: postgres
    # password: postgres
    # dbname: apirun
    # sslmode: disable
```

### Stateless Mode

```yaml
store:
  disabled: true  # No persistent state - useful for CI/CD
```

### Custom Table Names

```yaml
store:
  type: sqlite
  # Option 1: Prefix-based naming
  table_prefix: myapp  # creates: myapp_schema_migrations, myapp_migration_runs, myapp_stored_env

  # Option 2: Explicit names
  table_schema_migrations: custom_schema
  table_migration_runs: custom_runs
  table_stored_env: custom_env
```

## Health Check Configuration

### Basic Health Check

```yaml
wait:
  url: "{{.api_base}}/health"
  method: GET      # default: GET
  status: 200      # expected status code, default: 200
  timeout: 60s     # total wait time, default: 60s
  interval: 2s     # polling interval, default: 2s
```

### Advanced Health Check

```yaml
wait:
  url: "{{.api_base}}/api/v1/status"
  method: HEAD
  status: 200
  timeout: 120s
  interval: 5s
  headers:
    - name: Authorization
      value: "Bearer {{.health_check_token}}"
```

## HTTP Client Configuration

### TLS Settings

```yaml
client:
  insecure: false           # allow insecure TLS, default: false
  min_tls_version: "1.2"    # minimum TLS version: "1.0", "1.1", "1.2", "1.3"
  max_tls_version: "1.3"    # maximum TLS version
```

### Alternative TLS Version Format

```yaml
client:
  min_tls_version: "tls1.2"  # alternative format
  max_tls_version: "tls1.3"
```

## Logging Configuration

### Log Levels and Formats

```yaml
logging:
  level: info        # error, warn, info, debug
  format: text       # text, json, color
  color: true        # enable/disable colors (auto-detected if omitted)
```

### Sensitive Data Masking

```yaml
logging:
  level: debug
  format: json
  masking:
    enabled: true    # enable automatic masking of sensitive data
    # Automatically masks: password, token, secret, api_key, authorization headers
```

### Production Logging

```yaml
logging:
  level: info
  format: json      # structured logging for log aggregation
  masking:
    enabled: true
```

### Development Logging

```yaml
logging:
  level: debug
  format: color     # enhanced readability
  color: true
```

## Complete Configuration Example

```yaml
# Complete config.yaml example
auth:
  - type: basic
    name: admin_basic
    config:
      username: admin
      password: "{{.admin_password}}"

  - type: oauth2
    name: api_oauth
    config:
      client_id: "{{.oauth_client_id}}"
      client_secret: "{{.oauth_client_secret}}"
      token_url: "{{.oauth_token_url}}"

migrate_dir: ./migrations

env:
  - name: api_base
    value: https://api.example.com
  - name: environment
    value: production
  - name: admin_password
    valueFromEnv: ADMIN_PASSWORD
  - name: oauth_client_id
    valueFromEnv: OAUTH_CLIENT_ID
  - name: oauth_client_secret
    valueFromEnv: OAUTH_CLIENT_SECRET
  - name: oauth_token_url
    value: https://auth.example.com/oauth/token

store:
  type: postgres
  save_response_body: false
  postgres:
    dsn: "{{.database_url}}"
  table_prefix: myapp

wait:
  url: "{{.api_base}}/health"
  timeout: 30s
  interval: 2s

client:
  min_tls_version: "1.2"
  max_tls_version: "1.3"

logging:
  level: info
  format: json
  masking:
    enabled: true
```

## Environment-Specific Configurations

### Development (config-dev.yaml)

```yaml
auth:
  - type: basic
    name: dev_basic
    config:
      username: dev
      password: dev123

env:
  - name: api_base
    value: http://localhost:8080
  - name: environment
    value: development

store:
  type: sqlite
  sqlite:
    path: ./dev.db

logging:
  level: debug
  format: color

client:
  insecure: true  # allow self-signed certs in development
```

### Production (config-prod.yaml)

```yaml
auth:
  - type: oauth2
    name: prod_oauth
    config:
      client_id: "{{.oauth_client_id}}"
      client_secret: "{{.oauth_client_secret}}"
      token_url: "{{.oauth_token_url}}"

env:
  - name: api_base
    valueFromEnv: PRODUCTION_API_BASE
  - name: environment
    value: production

store:
  type: postgres
  postgres:
    dsn: "{{.database_url}}"

wait:
  url: "{{.api_base}}/health"
  timeout: 60s

logging:
  level: info
  format: json

client:
  min_tls_version: "1.3"
```

## Configuration Best Practices

1. **Use Environment Variables for Secrets**: Never hardcode passwords or tokens
2. **Environment-Specific Configs**: Separate configs for dev/staging/prod
3. **Template Variables**: Use `{{.variable}}` for dynamic values
4. **Health Checks**: Always configure health checks for production
5. **Structured Logging**: Use JSON format for production environments
6. **TLS Security**: Use TLS 1.3 minimum for production
7. **Store Configuration**: Use PostgreSQL for production, SQLite for development

## Validation Rules

- **Table Names**: Must match regex `^[a-zA-Z_][a-zA-Z0-9_]*$`
- **TLS Versions**: Must be one of "1.0", "1.1", "1.2", "1.3" or "tls1.0", "tls1.1", "tls1.2", "tls1.3"
- **Log Levels**: Must be one of "error", "warn", "info", "debug"
- **Log Formats**: Must be one of "text", "json", "color"
- **Store Types**: Must be "sqlite" or "postgres"