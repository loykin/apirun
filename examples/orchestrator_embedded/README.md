# Orchestrator Embedded Example

This example demonstrates how to use apirun's multi-stage orchestration feature programmatically from Go code using the public API.

## Overview

The example showcases a complete infrastructure deployment workflow with four dependent stages:

```
infrastructure → database → services → configuration
```

Each stage:
- Has its own config, migrations, and database state
- Depends on environment variables from previous stages
- Exports new variables for dependent stages

## Structure

```
orchestrator_embedded/
├── main.go                          # Main program with orchestrator usage
├── stages.yaml                      # Multi-stage configuration
├── infrastructure/
│   ├── config.yaml                  # Infrastructure stage config
│   └── migration/
│       └── 001_setup_vpc.yaml      # VPC setup migration
├── database/
│   ├── config.yaml                  # Database stage config
│   └── migration/
│       └── 001_create_database.yaml # Database creation migration
├── services/
│   ├── config.yaml                  # Services stage config
│   └── migration/
│       └── 001_deploy_services.yaml # Service deployment migration
└── configuration/
    ├── config.yaml                  # Configuration stage config
    └── migration/
        └── 001_configure_services.yaml # Service configuration migration
```

## Features Demonstrated

1. **Programmatic Orchestration**: Using `orchestrator.LoadFromFile()` and `ExecuteStages()`
2. **Dependency Management**: Automatic execution ordering based on `depends_on`
3. **Environment Variable Propagation**: Variables exported by parent stages become available to children
4. **Multiple Execution Modes**: Normal execution, dry-run, and status checking
5. **Independent State Management**: Each stage maintains its own SQLite database

## Environment Variable Flow

```
infrastructure stage:
  exports: vpc_id, region

database stage:
  imports: vpc_id, region (from infrastructure)
  exports: db_endpoint, db_credentials

services stage:
  imports: vpc_id, region (from infrastructure)
          db_endpoint, db_credentials (from database)
  exports: api_endpoint, service_token

configuration stage:
  imports: api_endpoint, service_token (from services)
  exports: config_id, final_status
```

## Usage

Run from the repository root:

### Normal Execution
```bash
go run ./examples/orchestrator_embedded
```

### Dry-run Mode
```bash
go run ./examples/orchestrator_embedded --dry-run
```

### Status Check
```bash
go run ./examples/orchestrator_embedded --status
```

## Expected Output

### Normal Execution
```
Running orchestrator with config: examples/orchestrator_embedded/stages.yaml
Starting multi-stage execution...
✅ All stages completed successfully!
```

### Dry-run Output
```
Dry-run validation with config: examples/orchestrator_embedded/stages.yaml
🔍 Execution plan (up):
From stage: <beginning>
To stage: <end>

📋 Stages to execute:
  • Note: Detailed execution plan requires orchestrator graph exposure

⚠️  This is a dry run - no changes will be made
```

### Status Output
```
Status check with config: examples/orchestrator_embedded/stages.yaml
✅ Configuration is valid
```

## Code Highlights

### Main Orchestrator Usage
```go
// Initialize orchestrator from config file
orch, err := orchestrator.LoadFromFile(configPath)
if err != nil {
    log.Fatalf("Failed to initialize orchestrator: %v", err)
}

// Execute all stages
err = orch.ExecuteStages(ctx, "", "") // Empty from/to means execute all stages
if err != nil {
    log.Fatalf("Stage execution failed: %v", err)
}
```

### Dry-run Execution
```go
// For dry-run, we just validate the configuration
_, err := orchestrator.LoadFromFile(configPath)
if err != nil {
    log.Fatalf("Configuration validation failed: %v", err)
}
```

### Status Checking
```go
// Get stage results after execution
results := orch.GetStageResults()
for stageName, result := range results {
    fmt.Printf("Stage %s: success=%v, duration=%v\n",
        stageName, result.Success, result.Duration)
}
```

## Integration with Library

This example shows how to integrate apirun's orchestration capabilities into larger Go applications:

1. **Embedded Deployment**: Include orchestration as part of application startup
2. **Custom Workflows**: Build complex deployment pipelines programmatically
3. **Status Monitoring**: Check deployment status from application code
4. **CI/CD Integration**: Use in automated deployment systems

## Advanced Usage

The orchestrator API supports additional options:

- **Partial Execution**: Execute specific stage ranges using `ExecuteStages(ctx, "from-stage", "to-stage")`
- **Custom Environment**: Override environment variables programmatically
- **Error Handling**: Fine-grained control over failure scenarios through stage configurations
- **Results Inspection**: Access detailed stage results and extracted variables

See the [Multi-Stage Orchestration documentation](../../README.md#multi-stage-orchestration) for more details.