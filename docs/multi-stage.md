# Multi-Stage Orchestration

Complete guide for managing complex workflows with dependent stages and automatic execution ordering.

## Overview

Multi-stage orchestration allows you to:
- **Manage Dependencies**: Define stage execution order through dependencies
- **Share Variables**: Pass environment variables between stages
- **Isolate State**: Each stage maintains independent migration history
- **Partial Execution**: Run specific stages or stage ranges
- **Failure Handling**: Configure behavior when stages fail

## Stage Configuration File

Create a `stages.yaml` file to define your multi-stage workflow:

```yaml
apiVersion: apirun/v1
kind: StageOrchestration

stages:
  - name: infrastructure
    config_path: infra/config.yaml
    depends_on: []
    env:
      region: us-west-2
      instance_type: t3.micro

  - name: database
    config_path: db/config.yaml
    depends_on: [infrastructure]
    env_from_stages:
      - stage: infrastructure
        vars: [vpc_id, subnet_id]
    env:
      db_engine: postgres

  - name: services
    config_path: services/config.yaml
    depends_on: [database]
    env_from_stages:
      - stage: infrastructure
        vars: [vpc_id]
      - stage: database
        vars: [db_endpoint, db_password]

  - name: configuration
    config_path: config/config.yaml
    depends_on: [services]
    env_from_stages:
      - stage: services
        vars: [api_url, auth_service_url]

global:
  env:
    project_name: my-project
    environment: production
  wait_between_stages: 10s
  max_concurrent_stages: 2
  rollback_on_failure: false
```

## Stage Properties

### Required Properties

- **name**: Unique identifier for the stage
- **config_path**: Path to the stage's config.yaml file

### Optional Properties

- **depends_on**: Array of stage names this stage depends on
- **env**: Stage-specific environment variables
- **env_from_stages**: Variables inherited from other stages
- **timeout**: Maximum execution time for this stage
- **on_failure**: Behavior when stage fails (`stop`, `continue`, `skip_dependents`)
- **condition**: Go template condition for conditional execution

### Global Settings

- **env**: Global environment variables available to all stages
- **wait_between_stages**: Delay between stage executions
- **max_concurrent_stages**: Maximum stages to run in parallel
- **rollback_on_failure**: Whether to rollback previous stages on failure

## Environment Variable Flow

### Variable Export in Migrations

```yaml
# infrastructure/migration/001_create_vpc.yaml
up:
  name: create VPC
  request:
    method: POST
    url: "{{.api_base}}/vpc"
    body: |
      {
        "cidr": "{{.vpc_cidr}}",
        "region": "{{.region}}"
      }
  response:
    result_code: ["201"]
    env_from:
      vpc_id: "vpc.id"           # Extract VPC ID from response
      subnet_id: "vpc.subnets.0.id"  # Extract first subnet ID
```

### Variable Import in Stage Configuration

```yaml
# database stage configuration
stages:
  - name: database
    depends_on: [infrastructure]
    env_from_stages:
      - stage: infrastructure
        vars: [vpc_id, subnet_id]  # Import these variables
```

### Variable Usage in Dependent Migrations

```yaml
# database/migration/001_create_database.yaml
up:
  name: create database
  request:
    method: POST
    url: "{{.api_base}}/database"
    body: |
      {
        "vpc_id": "{{.vpc_id}}",        # From infrastructure stage
        "subnet_id": "{{.subnet_id}}",  # From infrastructure stage
        "engine": "{{.db_engine}}"      # From stage env
      }
  response:
    env_from:
      db_endpoint: "database.endpoint"
      db_password: "database.password"
```

## Execution Commands

### Basic Execution

```bash
# Execute all stages in dependency order
apirun stages up --config stages.yaml

# Execute specific stage (requires dependencies to be completed)
apirun stages up --stage database --config stages.yaml

# Execute stage range
apirun stages up --from infrastructure --to services --config stages.yaml
```

### Planning and Validation

```bash
# Validate configuration and dependencies
apirun stages validate --config stages.yaml

# Show execution plan without running
apirun stages up --dry-run --config stages.yaml

# Check current status
apirun stages status --config stages.yaml

# Verbose status with variable details
apirun stages status --verbose --config stages.yaml
```

### Rollback

```bash
# Rollback all stages
apirun stages down --config stages.yaml

# Rollback specific stage range
apirun stages down --from services --to database --config stages.yaml
```

## Advanced Features

### Conditional Execution

```yaml
stages:
  - name: production-monitoring
    config_path: monitoring/config.yaml
    condition: "{{.environment}} == 'production'"
    depends_on: [services]

  - name: development-tools
    config_path: devtools/config.yaml
    condition: "{{.environment}} == 'development'"
    depends_on: [services]
```

### Failure Handling

```yaml
stages:
  - name: optional-analytics
    config_path: analytics/config.yaml
    depends_on: [services]
    on_failure: continue        # Continue with other stages if this fails
    timeout: 300s

  - name: critical-security
    config_path: security/config.yaml
    depends_on: [services]
    on_failure: stop           # Stop entire workflow if this fails (default)

  - name: dependent-service
    config_path: dependent/config.yaml
    depends_on: [optional-analytics]
    on_failure: skip_dependents  # Skip stages that depend on this
```

### Parallel Execution

```yaml
global:
  max_concurrent_stages: 3  # Run up to 3 independent stages in parallel

stages:
  - name: service-a
    config_path: service-a/config.yaml
    depends_on: [infrastructure]

  - name: service-b
    config_path: service-b/config.yaml
    depends_on: [infrastructure]  # Can run in parallel with service-a

  - name: service-c
    config_path: service-c/config.yaml
    depends_on: [infrastructure]  # Can run in parallel with service-a and service-b

  - name: integration
    config_path: integration/config.yaml
    depends_on: [service-a, service-b, service-c]  # Runs after all services complete
```

## Stage Directory Structure

### Recommended Layout

```
project/
├── stages.yaml                    # Multi-stage configuration
├── infrastructure/
│   ├── config.yaml               # Infrastructure stage config
│   └── migration/
│       ├── 001_create_vpc.yaml
│       └── 002_setup_security.yaml
├── database/
│   ├── config.yaml               # Database stage config
│   └── migration/
│       ├── 001_create_db.yaml
│       └── 002_setup_users.yaml
├── services/
│   ├── config.yaml               # Services stage config
│   └── migration/
│       ├── 001_deploy_api.yaml
│       ├── 002_deploy_auth.yaml
│       └── 003_configure_lb.yaml
└── configuration/
    ├── config.yaml               # Configuration stage config
    └── migration/
        ├── 001_setup_monitoring.yaml
        └── 002_configure_alerts.yaml
```

### Individual Stage Configuration

Each stage has its own standard apirun configuration:

```yaml
# infrastructure/config.yaml
migrate_dir: ./migration

auth:
  - type: basic
    name: aws_api
    config:
      username: "{{.aws_access_key}}"
      password: "{{.aws_secret_key}}"

env:
  - name: aws_access_key
    valueFromEnv: AWS_ACCESS_KEY_ID
  - name: aws_secret_key
    valueFromEnv: AWS_SECRET_ACCESS_KEY
  - name: vpc_cidr
    value: 10.0.0.0/16

store:
  type: sqlite
  sqlite:
    path: ./infrastructure_state.db
```

## State Management

### Independent Stage State

Each stage maintains its own migration state:

```bash
infrastructure/infrastructure_state.db  # Infrastructure migration history
database/database_state.db             # Database migration history
services/services_state.db             # Services migration history
configuration/configuration_state.db   # Configuration migration history
```

### Shared Environment Variables

Environment variables flow between stages but state remains isolated:

```
[Infrastructure] → extracts vpc_id, subnet_id
       ↓
[Database] → receives vpc_id, subnet_id → extracts db_endpoint, db_password
       ↓
[Services] → receives vpc_id, db_endpoint, db_password → extracts api_url
       ↓
[Configuration] → receives api_url
```

## Best Practices

### 1. Dependency Design

```yaml
# Good: Clear linear dependencies
infrastructure → database → services → configuration

# Good: Parallel independent services
infrastructure → [service-a, service-b, service-c] → integration

# Avoid: Complex cross-dependencies
# infrastructure → database ↘
#       ↓                    ↘ services
# monitoring ← analytics ←    ↗
```

### 2. Variable Naming

```yaml
# Use consistent, descriptive variable names
env_from:
  vpc_id: "vpc.id"                    # Good
  database_endpoint: "db.endpoint"    # Good
  x: "response.data"                  # Bad - unclear
```

### 3. Error Handling Strategy

```yaml
# Critical path stages - stop on failure
stages:
  - name: infrastructure
    on_failure: stop

  - name: database
    on_failure: stop

# Optional features - continue on failure
  - name: monitoring
    on_failure: continue

  - name: analytics
    on_failure: skip_dependents
```

### 4. Development Workflow

```bash
# 1. Validate configuration
apirun stages validate

# 2. Test with dry run
apirun stages up --dry-run

# 3. Test individual stages
apirun stages up --stage infrastructure
apirun stages up --stage database

# 4. Test stage ranges
apirun stages up --from infrastructure --to database

# 5. Full execution
apirun stages up
```

## Troubleshooting

### Common Issues

#### Dependency Not Executed

```bash
Error: stage services failed: dependent stage database has not been executed

# Solution: Run dependencies first
apirun stages up --from infrastructure --to database
apirun stages up --stage services
```

#### Variable Not Found

```bash
Error: template: executing "template" at <.vpc_id>: map has no entry for key "vpc_id"

# Check if parent stage exports the variable
apirun stages status --verbose | grep vpc_id

# Check parent stage migration response.env_from section
```

#### Circular Dependencies

```bash
Error: circular dependency detected: infrastructure → database → infrastructure

# Fix: Remove circular dependency in stages.yaml
```

### Debugging Commands

```bash
# Check execution plan
apirun stages up --dry-run

# Verbose status with variable details
apirun stages status --verbose

# Individual stage status
apirun status --config infrastructure/config.yaml

# Validate configuration
apirun stages validate
```

## Migration Lifecycle in Multi-Stage Context

### State-Based System

- Migration execution is determined by database records, not file system state
- Each stage maintains independent migration history
- Adding new migration files to any stage triggers execution
- File deletion doesn't affect already-applied migrations

### Development-Friendly Behavior

```bash
# After all stages complete, you can still add migrations
echo "New migration added to infrastructure/migration/003_new_feature.yaml"

# Running stages again will execute the new migration
apirun stages up
# Result: Only the new migration in infrastructure stage executes
```

### Variable Dependency Considerations

When adding new migrations that export variables:

1. **Dependent stages may need updates** to use new variables
2. **Template errors occur** if dependent migrations reference non-existent variables
3. **Use defensive templates** with defaults: `{{.new_var | default "fallback"}}`

## Performance Optimization

### Parallel Execution

```yaml
global:
  max_concurrent_stages: 3  # Adjust based on resource constraints

# Stages with same dependencies can run in parallel
stages:
  - name: monitoring
    depends_on: [infrastructure]
  - name: logging
    depends_on: [infrastructure]    # Runs in parallel with monitoring
  - name: analytics
    depends_on: [infrastructure]    # Runs in parallel with monitoring and logging
```

### Resource Management

```yaml
stages:
  - name: heavy-computation
    timeout: 1800s              # 30 minutes for complex operations
    env:
      compute_resources: large

  - name: quick-config
    timeout: 60s                # 1 minute for simple configuration
```