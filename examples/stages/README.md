# Multi-Stage Orchestration Example

This example demonstrates the new multi-stage orchestration feature in apirun, which allows you to manage complex deployment workflows with multiple dependent stages.

## Directory Structure

```
examples/stages/
├── stages.yaml                    # Main orchestration configuration
├── infrastructure/                # Infrastructure provisioning stage
│   ├── config.yaml                # Stage-specific configuration
│   └── migration/                 # Infrastructure migrations
│       ├── 001_create_vpc.yaml
│       └── 002_create_database.yaml
├── services/                      # Service deployment stage
│   ├── config.yaml
│   └── migration/
│       ├── 001_deploy_auth_service.yaml
│       └── 002_deploy_api_service.yaml
└── configuration/                 # Application configuration stage
    ├── config.yaml
    └── migration/
        ├── 001_create_admin_user.yaml
        └── 002_setup_default_settings.yaml
```

## How It Works

### 1. Stage Dependencies
- **Infrastructure**: Creates VPC, database, and other foundational resources
- **Services**: Deploys applications (depends on infrastructure)
- **Configuration**: Sets up users and application settings (depends on services)

### 2. Environment Variable Propagation
Each stage can extract variables that are automatically passed to dependent stages:

- Infrastructure extracts: `vpc_id`, `db_endpoint`, `security_group_id`
- Services extracts: `auth_service_url`, `api_service_url`
- Configuration uses variables from previous stages

### 3. Independent Configuration
Each stage has its own:
- Migration directory
- Authentication configuration
- Store (database) configuration
- Environment variables

## Usage Examples

### Execute All Stages
```bash
# Run the complete workflow
apirun stages up --config examples/stages/stages.yaml
```

### Execute Specific Stages
```bash
# Run only infrastructure stage
apirun stages up --stage infrastructure --config examples/stages/stages.yaml

# Run from services to configuration
apirun stages up --from services --to configuration --config examples/stages/stages.yaml

# Run only services stage
apirun stages up --from services --to services --config examples/stages/stages.yaml
```

### Rollback Operations
```bash
# Rollback all stages (in reverse order)
apirun stages down --config examples/stages/stages.yaml

# Rollback from configuration to services
apirun stages down --from configuration --to services --config examples/stages/stages.yaml

# Rollback only the configuration stage
apirun stages down --stage configuration --config examples/stages/stages.yaml
```

### Status and Validation
```bash
# Check status of all stages
apirun stages status --config examples/stages/stages.yaml

# Detailed status with environment variables
apirun stages status --verbose --config examples/stages/stages.yaml

# Validate configuration without running
apirun stages validate --config examples/stages/stages.yaml

# Dry run to see execution plan
apirun stages up --dry-run --config examples/stages/stages.yaml
```

## Configuration Features

### Stage-Level Features
- **Timeouts**: Each stage can have execution timeouts
- **Failure Handling**: Configure what happens on stage failure (`stop`, `continue`, `skip_dependents`)
- **Conditional Execution**: Run stages based on conditions
- **Environment Variables**: Stage-specific and inherited from other stages

### Global Features
- **Wait Between Stages**: Configurable delays between stage execution
- **Rollback on Failure**: Automatic rollback if any stage fails
- **Concurrent Execution**: Control maximum parallel stages

## Benefits

1. **Modularity**: Each stage is independent and reusable
2. **Dependency Management**: Automatic ordering based on dependencies
3. **Environment Isolation**: Each stage can have its own configuration
4. **Partial Operations**: Run or rollback specific stages only
5. **Visibility**: Clear status and progress tracking
6. **Safety**: Validation and dry-run capabilities

## Dependency Management Deep Dive

### 🔗 Understanding Stage Dependencies

#### Execution Flow
```
infrastructure → services → configuration
     ↓             ↓           ↓
   vpc_id      auth_service  admin_user
 db_endpoint     api_url      settings
```

#### Dependency Rules
- **Sequential Execution**: Stages execute in dependency order (topological sort)
- **Prerequisite Check**: Dependent stages verify all parent stages completed successfully
- **Environment Inheritance**: Child stages automatically receive parent stage variables

### 📊 State Management

#### Migration State Isolation
Each stage maintains independent migration state:
```bash
infrastructure/migrations/    # State tracked in infra database
services/migrations/          # State tracked in services database
configuration/migrations/     # State tracked in config database
```

#### Adding Stages Mid-Development
✅ **Safe**: Add new stage between existing ones
```yaml
# Before: infra → config
# After:  infra → services → config
stages:
  - name: infrastructure
    # ... existing config

  - name: services          # ← New stage added
    config_path: services/config.yaml
    depends_on: [infrastructure]
    env_from_stages:
      - stage: infrastructure
        vars: [vpc_id, db_endpoint]

  - name: configuration
    depends_on: [services]   # ← Updated dependency
```

⚠️ **Caution**: Changing existing dependencies may require state cleanup

### 🔄 Migration File Lifecycle

#### Adding Migrations
- **Existing Stages**: New migration files are automatically detected and executed
- **Version Ordering**: Files execute in numerical order (001, 002, 003...)
- **State Tracking**: Applied migrations are recorded in each stage's database

#### Removing Migrations
- **Safe Removal**: Deleting migration files doesn't affect applied state
- **State Persistence**: Migration history remains in database
- **Development Workflow**: Safe to delete/add files during development

#### Template Variables in Dependencies
```yaml
# Parent stage (infrastructure) migration
response:
  env_from:
    vpc_id: "json.vpc.id"
    db_endpoint: "json.database.endpoint"

# Child stage (services) migration
body: |
  {
    "vpc_id": "{{.vpc_id}}",           # Available from parent
    "db_endpoint": "{{.db_endpoint}}"  # Available from parent
  }
```

### 🚨 Common Pitfalls & Solutions

#### Problem: Template Variable Not Found
```
Error: template: gotmpl:3:18: executing "gotmpl" at <.vpc_id>: map has no entry for key "vpc_id"
```
**Solution**: Ensure parent stage exports the variable in `response.env_from`

#### Problem: Dependent Stage Not Executed
```
Error: dependent stage infrastructure has not been executed
```
**Solutions**:
```bash
# Option 1: Run from parent stage
apirun stages up --from infrastructure --to services

# Option 2: Run all dependencies first
apirun stages up --to infrastructure
apirun stages up --stage services
```

#### Problem: Variable Not Found Warning
```
WARN: variable not found in dependent stage preprocessing variable=processed_data
```
**Root Cause**: Parent stage migration didn't extract the expected variable
**Solution**: Check parent stage's migration `response.env_from` section

## Real-World Use Cases

- **Infrastructure as Code**: Provision cloud resources in stages
- **Microservice Deployment**: Deploy services in dependency order
- **Database Migrations**: Schema, data, and configuration updates
- **Environment Setup**: Development, staging, and production workflows
- **Disaster Recovery**: Restore systems in proper sequence

This orchestration approach scales from simple workflows to complex enterprise deployments while maintaining clarity and safety.