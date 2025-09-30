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

## Real-World Use Cases

- **Infrastructure as Code**: Provision cloud resources in stages
- **Microservice Deployment**: Deploy services in dependency order
- **Database Migrations**: Schema, data, and configuration updates
- **Environment Setup**: Development, staging, and production workflows
- **Disaster Recovery**: Restore systems in proper sequence

This orchestration approach scales from simple workflows to complex enterprise deployments while maintaining clarity and safety.