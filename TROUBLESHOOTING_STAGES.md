# Multi-Stage Orchestration Troubleshooting Guide

This guide provides solutions for common issues encountered when using apirun's multi-stage orchestration feature.

## Quick Diagnosis Commands

```bash
# Validate configuration
apirun stages validate --config stages.yaml

# Check execution plan
apirun stages up --dry-run --config stages.yaml

# View detailed status
apirun stages status --verbose --config stages.yaml

# Check individual stage status
apirun status --config stage/config.yaml
```

## Common Error Patterns

### 1. Dependency Execution Errors

#### Error: "dependent stage X has not been executed"
```
Error: stage preprocessing failed: failed to build environment for stage preprocessing: dependent stage setup has not been executed
```

**Root Cause**: Attempting to run a stage without executing its dependencies first.

**Solutions**:
```bash
# Option 1: Run from dependency chain start
apirun stages up --from setup --to preprocessing

# Option 2: Run all dependencies first
apirun stages up --to setup
apirun stages up --stage preprocessing

# Option 3: Run complete chain
apirun stages up  # Executes all stages in order
```

**Prevention**:
```yaml
# Use --dry-run to check execution plan
apirun stages up --dry-run
# Output shows: stages="[setup preprocessing validation cleanup]"
```

### 2. Template Variable Errors

#### Error: "map has no entry for key"
```
Error: template: gotmpl:3:18: executing "gotmpl" at <.vpc_id>: map has no entry for key "vpc_id"
```

**Root Cause**: Migration template references a variable that wasn't exported by parent stage.

**Diagnosis**:
```bash
# Check what variables parent stage exports
apirun stages status --verbose | grep -A 5 "parent-stage"

# Verify parent stage migration
cat parent-stage/migration/*.yaml | grep -A 10 "env_from:"
```

**Solutions**:
```yaml
# Option 1: Fix parent stage migration to export variable
response:
  env_from:
    vpc_id: "json.vpc.id"  # Add missing export

# Option 2: Use template default in child migration
body: |
  {
    "vpc_id": "{{.vpc_id | default "default-vpc-id"}}"
  }

# Option 3: Remove template dependency
body: |
  {
    "vpc_id": "hardcoded-value"
  }
```

#### Warning: "variable not found in dependent stage"
```
WARN: variable not found in dependent stage preprocessing variable=processed_data
```

**Root Cause**: Stage configuration expects a variable that parent stage doesn't export.

**Diagnosis**:
```bash
# Check stage configuration
grep -A 10 "env_from_stages:" stages.yaml

# Check parent stage exports
apirun stages status --verbose | grep "extracted_vars"
```

**Solutions**:
```yaml
# Option 1: Update parent stage to export variable
# In parent-stage/migration/001_process.yaml:
response:
  env_from:
    processed_data: "json.result"

# Option 2: Remove unused variable from stage config
# In stages.yaml:
env_from_stages:
  - stage: parent-stage
    vars: [setup_id]  # Remove processed_data
```

### 3. Configuration Errors

#### Error: "config file not found"
```
Error: stage infrastructure: config file not found: infrastructure/config.yaml
```

**Root Cause**: Missing stage configuration file.

**Solutions**:
```bash
# Create missing config file
mkdir -p infrastructure
cat > infrastructure/config.yaml << 'EOF'
migrate_dir: migration
store:
  driver: sqlite
  path: ./infra_state.db
EOF
```

#### Error: "duplicate stage name"
```
Error: duplicate stage name: infrastructure
```

**Root Cause**: Two stages have the same name in `stages.yaml`.

**Solution**:
```yaml
# Fix: Ensure unique stage names
stages:
  - name: infrastructure-setup  # Was: infrastructure
  - name: infrastructure-config # Was: infrastructure
```

#### Error: "stage X: dependency Y not found"
```
Error: stage services: dependency infrastructure not found
```

**Root Cause**: Stage depends on a non-existent stage.

**Solutions**:
```yaml
# Option 1: Fix dependency name
depends_on: [infrastructure-setup]  # Correct name

# Option 2: Add missing stage
stages:
  - name: infrastructure  # Add missing stage

# Option 3: Remove invalid dependency
depends_on: []  # Remove dependency
```

### 4. Migration State Issues

#### Issue: Migration not executing despite new file
```bash
# Check if migration was already applied
apirun status --config stage/config.yaml

# Output shows: current version 5, but file is 003_something.yaml
```

**Root Cause**: Version number conflict or gap.

**Solutions**:
```bash
# Option 1: Use next sequential version
mv stage/migration/003_something.yaml stage/migration/006_something.yaml

# Option 2: Check for hidden applied migrations
ls stage/migration/  # Verify all files
sqlite3 stage_state.db "SELECT * FROM schema_migrations;"
```

#### Issue: Stage executed but no variables extracted
```
INFO: stage executed successfully stage=setup extracted_vars=0
```

**Root Cause**: Migration doesn't have `response.env_from` section.

**Solution**:
```yaml
# Add to migration file
response:
  result_code: ["200"]
  env_from:
    setup_id: "json.id"
    vpc_id: "json.vpc.id"
```

### 5. Partial Execution Issues

#### Error: Partial execution skips expected stages
```bash
apirun stages up --from services --to configuration
# Only executes services, skips configuration
```

**Root Cause**: `--to` is exclusive when dependency exists.

**Solutions**:
```bash
# Include end stage in range
apirun stages up --from services  # Execute services and all after

# Or use specific stage
apirun stages up --stage configuration  # Requires services to be executed first
```

## Development Workflow Issues

### Issue: Adding new stage breaks existing workflow

**Scenario**: Added `preprocessing` between `setup` and `validation`

**Error**: `validation` stage fails with template errors

**Solution Process**:
1. **Update dependencies**:
```yaml
# stages.yaml
- name: validation
  depends_on: [preprocessing]  # Was: [setup]
```

2. **Update variable sources**:
```yaml
env_from_stages:
  - stage: setup
    vars: [setup_id]
  - stage: preprocessing
    vars: [processed_data]
```

3. **Test incrementally**:
```bash
apirun stages up --from setup --to preprocessing  # Test new stage
apirun stages up --from preprocessing --to validation  # Test integration
```

### Issue: Renaming stages breaks environment inheritance

**Problem**: Renamed `infrastructure` to `infra`, now `services` can't find variables.

**Root Cause**: Environment variables are associated with stage names.

**Solutions**:
```bash
# Option 1: Clean start (development only)
rm -f */state.db  # Remove all stage databases
apirun stages up  # Fresh execution

# Option 2: Update all references
grep -r "infrastructure" stages.yaml env_from_stages
# Update all references to new name
```

## Performance Issues

### Issue: Stages taking too long

**Diagnosis**:
```bash
# Check individual stage timing
apirun stages status --verbose | grep "duration"

# Profile specific stage
time apirun up --config slow-stage/config.yaml
```

**Solutions**:
```yaml
# Add timeouts to prevent hanging
stages:
  - name: slow-stage
    timeout: 300s  # 5 minute timeout

# Add parallelization
global:
  max_concurrent_stages: 3  # Run independent stages in parallel
```

### Issue: Too many dependency checks

**Symptom**: Warnings about missing variables but execution works

**Optimization**:
```yaml
# Only request variables you actually use
env_from_stages:
  - stage: parent
    vars: [vpc_id]  # Remove unused variables
```

## Testing and Validation Strategies

### 1. Incremental Testing
```bash
# Test configuration only
apirun stages validate

# Test execution plan
apirun stages up --dry-run

# Test single stage
apirun stages up --stage target-stage

# Test stage range
apirun stages up --from start --to end
```

### 2. Variable Validation
```bash
# Check variable flow
apirun stages status --verbose | grep -E "(extracted_vars|stage)"

# Test template rendering
echo "Test: {{.vpc_id}}" | apirun migrate up --config stage/config.yaml --dry-run
```

### 3. State Verification
```bash
# Check migration state
apirun status --config stage/config.yaml

# Verify database content
sqlite3 stage_state.db "SELECT version, applied_at FROM schema_migrations ORDER BY version;"
```

## Recovery Procedures

### 1. Reset Stage State
```bash
# Caution: This removes all migration history
rm stage/state.db
apirun stages up --stage target-stage
```

### 2. Fix Broken Dependencies
```bash
# Run dependencies first
apirun stages up --to parent-stage

# Then run dependent stage
apirun stages up --stage child-stage
```

### 3. Variable Inheritance Recovery
```bash
# Re-run parent stage to regenerate variables
apirun stages up --stage parent-stage

# Then continue with dependent stages
apirun stages up --from child-stage
```

## Prevention Best Practices

### 1. Configuration Validation
```bash
# Always validate before execution
apirun stages validate

# Use dry-run for testing
apirun stages up --dry-run
```

### 2. Documentation
```yaml
# Document stage dependencies and variables
# stages.yaml comments:
stages:
  - name: services
    # Depends on: infrastructure (provides vpc_id, db_endpoint)
    # Exports: auth_service_url, api_service_url
```

### 3. Testing
```bash
# Test in isolated environment first
cp -r production-stages test-stages
cd test-stages && apirun stages up
```

### 4. Monitoring
```bash
# Regular status checks
apirun stages status --verbose

# Log migration results
apirun stages up 2>&1 | tee execution.log
```

For more detailed information, see [MIGRATION_LIFECYCLE.md](MIGRATION_LIFECYCLE.md) and the main [README.md](README.md) Multi-Stage Orchestration section.