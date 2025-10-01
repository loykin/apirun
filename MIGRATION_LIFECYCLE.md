# Migration File Lifecycle in Multi-Stage Context

This document explains how migration files behave in multi-stage orchestration environments, including dependency management, state tracking, and development workflows.

## Overview

apirun uses a **state-based migration system** where migration execution is determined by database records, not file system state. This approach ensures consistency across different environments and supports dynamic development workflows.

## Single Stage Migration Lifecycle

### File Structure
```
stage/
├── config.yaml
└── migration/
    ├── 001_initial_setup.yaml
    ├── 002_add_users.yaml
    └── 003_configure_settings.yaml
```

### State Tracking
- **Database Records**: Each applied migration is recorded with version number and timestamp
- **Version Ordering**: Migrations execute in numerical order (001, 002, 003...)
- **Idempotent**: Re-running migrations only executes new/unapplied versions

### Operations

#### ✅ Safe Operations
- **Add New Files**: `004_new_feature.yaml` - automatically detected and executed
- **Delete Files**: Remove `002_add_users.yaml` - already applied migrations remain in database
- **Modify Unapplied**: Change content of future migrations before they're applied

#### ⚠️ Risky Operations
- **Modify Applied**: Changing already-applied migration content (database state remains unchanged)
- **Renumber Files**: Changing version numbers can cause confusion

## Multi-Stage Migration Lifecycle

### Dependency Chain Example
```
setup → preprocessing → validation → cleanup
```

### State Isolation
Each stage maintains **independent migration state**:
```bash
setup/migration/         # State in setup database
preprocessing/migration/ # State in preprocessing database
validation/migration/    # State in validation database
cleanup/migration/       # State in cleanup database
```

### Environment Variable Flow
```yaml
# setup/migration/001_setup.yaml
response:
  env_from:
    setup_id: "json.id"
    vpc_id: "json.vpc.id"

# preprocessing/migration/001_process.yaml
body: |
  {
    "setup_id": "{{.setup_id}}",  # From setup stage
    "vpc_id": "{{.vpc_id}}"       # From setup stage
  }
response:
  env_from:
    processed_data: "json.result"

# validation/migration/001_validate.yaml
body: |
  {
    "setup_id": "{{.setup_id}}",        # From setup stage
    "processed_data": "{{.processed_data}}" # From preprocessing stage
  }
```

## Development Scenarios

### Scenario 1: Adding Migration to Existing Stage

#### Before
```
stages: setup → validation → cleanup
setup/migration/: [001_setup.yaml]
```

#### After
```
stages: setup → validation → cleanup
setup/migration/: [001_setup.yaml, 002_additional_setup.yaml]
```

#### Behavior
- Next execution applies `002_additional_setup.yaml`
- New environment variables from `002_additional_setup.yaml` become available to dependent stages
- Dependent stages may need migration updates to use new variables

### Scenario 2: Adding New Stage in Middle

#### Before
```
stages: setup → validation → cleanup
```

#### After
```
stages: setup → preprocessing → validation → cleanup
```

#### Required Changes
```yaml
# stages.yaml updates
- name: preprocessing
  config_path: preprocessing/config.yaml
  depends_on: [setup]
  env_from_stages:
    - stage: setup
      vars: [setup_id, vpc_id]

- name: validation
  depends_on: [preprocessing]  # Changed from [setup]
  env_from_stages:
    - stage: setup
      vars: [setup_id, vpc_id]
    - stage: preprocessing      # New dependency
      vars: [processed_data]
```

#### Migration Considerations
- **Template Dependencies**: Validation stage migrations using `{{.processed_data}}` must be updated
- **Execution Order**: New topological order: setup → preprocessing → validation → cleanup
- **State Independence**: Each stage's migration history remains separate

### Scenario 3: Removing Migration Files

#### Operation
```bash
rm setup/migration/002_additional_setup.yaml
```

#### Behavior
- **Database State**: `version=2` remains recorded as applied
- **Next Execution**: Skips version 2, continues with version 3+ if they exist
- **Dependent Stages**: May receive warnings if they expect variables from deleted migration

## Template Variable Dependencies

### Variable Export
```yaml
# Parent stage migration
response:
  env_from:
    vpc_id: "json.vpc.id"
    db_endpoint: "json.database.endpoint"
    secret_key: "json.auth.secret"
```

### Variable Import
```yaml
# Child stage configuration
env_from_stages:
  - stage: parent-stage
    vars: [vpc_id, db_endpoint, secret_key]
```

### Variable Usage
```yaml
# Child stage migration
body: |
  {
    "vpc_id": "{{.vpc_id}}",
    "db_endpoint": "{{.db_endpoint}}",
    "secret": "{{.secret_key}}"
  }
```

### Error Scenarios
```
# Error: Template variable not found
Error: template: gotmpl:3:18: executing "gotmpl" at <.vpc_id>: map has no entry for key "vpc_id"

# Cause: Parent stage didn't export vpc_id
# Solution: Update parent stage migration response.env_from
```

## State Management Best Practices

### 1. Database Separation
```yaml
# Recommended: Separate databases per stage
stages:
  - name: infrastructure
    store:
      driver: sqlite
      path: ./infra_state.db

  - name: services
    store:
      driver: sqlite
      path: ./services_state.db
```

### 2. Version Control Strategy
```bash
# Track migration files in version control
git add setup/migration/001_setup.yaml
git commit -m "Add initial setup migration"

# Database state files are environment-specific
echo "*.db" >> .gitignore
```

### 3. Development Workflow
```bash
# 1. Validate configuration changes
apirun stages validate

# 2. Test with dry-run
apirun stages up --dry-run

# 3. Execute incrementally
apirun stages up --from changed-stage

# 4. Verify results
apirun stages status --verbose
```

## Troubleshooting Migration Issues

### Issue: Migration Not Executing
```bash
# Check stage migration state
apirun status --config stage/config.yaml

# Check if file is properly named
ls stage/migration/  # Should be: 001_name.yaml, 002_name.yaml

# Verify execution order
apirun stages up --dry-run
```

### Issue: Template Rendering Errors
```bash
# Check environment variable propagation
apirun stages status --verbose

# Verify parent stage exports
grep -A 10 "env_from:" parent-stage/migration/*.yaml

# Test partial execution
apirun stages up --from parent-stage --to child-stage
```

### Issue: Dependency Conflicts
```bash
# Validate dependency graph
apirun stages validate

# Check execution plan
apirun stages up --dry-run

# Resolve by updating depends_on relationships
```

## Migration File Best Practices

### 1. Naming Convention
```
001_descriptive_name.yaml    # Good
002_add_user_authentication.yaml
003_configure_database_settings.yaml

migration.yaml               # Bad - no version
setup.yaml                   # Bad - no version
```

### 2. Template Safety
```yaml
# Defensive templates with defaults
body: |
  {
    "vpc_id": "{{.vpc_id | default "default-vpc"}}",
    "optional_field": "{{.optional_var | default ""}}"
  }
```

### 3. Environment Variable Documentation
```yaml
# Document expected variables in migration comments
# Requires: setup_id (from setup stage), vpc_id (from infrastructure stage)
# Exports: processed_data, validation_status
up:
  name: process-and-validate
  # ... migration content
```

This lifecycle model ensures that multi-stage orchestration remains predictable and maintainable while supporting dynamic development workflows.