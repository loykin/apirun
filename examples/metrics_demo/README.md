# Enhanced Migration Logging Demo

This example demonstrates the enhanced logging capabilities of apirun, providing better visibility into migration progress and performance.

## Enhanced Logging Features

### 1. Migration Progress Tracking
```
time=2025-09-18T01:56:16.530+09:00 level=INFO msg="starting migration up" component=migrator target_version="" dir=./migration dry_run=""
time=2025-09-18T01:56:16.531+09:00 level=INFO msg="applying migration" component=migrator version=1 file="001_create_users.yaml"
time=2025-09-18T01:56:16.545+09:00 level=INFO msg="migration up completed" component=migrator applied_count=3 duration_ms=15 dry_run=""
```

### 2. HTTP Request Monitoring
```
time=2025-09-18T01:56:16.532+09:00 level=INFO msg="HTTP response" component=httpc method=POST url="https://api.example.com/users" status_code=201 duration_ms=234
```

### 3. Individual Migration Steps
- **Start Time**: When each migration begins
- **Version & File**: Which migration is being applied/rolled back
- **Completion Summary**: Total count and duration

## What's Logged

### Migration Operations
- **Start/End Times**: Complete migration duration
- **Applied Count**: Number of migrations processed
- **Individual Steps**: Each migration file being processed
- **Dry Run Status**: Whether running in simulation mode

### HTTP Requests
- **Request Details**: Method, URL
- **Response Info**: Status code, response time
- **Performance**: Request duration in milliseconds

### Authentication
- **Token Acquisition**: When auth tokens are obtained
- **Provider Type**: Which auth provider was used
- **Success/Failure**: Authentication status

## Benefits

### 🎯 **Practical Monitoring**
Unlike complex metrics systems (Prometheus, etc.), this provides:
- **Simple Progress Tracking** for one-time migration operations
- **Immediate Feedback** during migration execution
- **Performance Insights** without external dependencies

### 📊 **Right-Sized Solution**
Perfect for **CLI migration tools** that need:
- ✅ Progress visibility
- ✅ Performance timing
- ✅ Error context
- ❌ NOT complex dashboards
- ❌ NOT persistent metrics storage
- ❌ NOT real-time alerting

### 🔧 **Zero Configuration**
- Works out-of-the-box
- No external services required
- Structured logging compatible with log aggregation tools

## Running the Demo

```bash
go run ./examples/metrics_demo
```

## Integration

This enhanced logging is automatically available in all apirun operations:

```go
migrator := apirun.Migrator{
    Dir: "./migrations",
    Env: env,
}

// Automatically logs progress, timing, and HTTP requests
results, err := migrator.MigrateUp(ctx, 0)
```

## Comparison: When NOT to Use Prometheus

| Use Case | apirun Enhanced Logging | Prometheus |
|----------|----------------------------|------------|
| **CLI Migration Tool** | ✅ Perfect fit | ❌ Overkill |
| **One-time Operations** | ✅ Simple & effective | ❌ Unnecessary complexity |
| **Progress Tracking** | ✅ Built-in | ❌ Requires setup |
| **24/7 Web Service** | ❌ Insufficient | ✅ Ideal |
| **Real-time Dashboards** | ❌ Not designed for this | ✅ Perfect |
| **Historical Analysis** | ❌ Logs only | ✅ Time series DB |

## Log Output Examples

### Successful Migration
```
time=2025-09-18T01:56:16.530+09:00 level=INFO msg="starting migration up" component=migrator target_version=0 dir=./migrations dry_run=false
time=2025-09-18T01:56:16.531+09:00 level=INFO msg="applying migration" component=migrator version=1 file="001_create_users.yaml"  
time=2025-09-18T01:56:16.532+09:00 level=INFO msg="HTTP response" component=httpc method=POST url="https://api.example.com/users" status_code=201 duration_ms=234
time=2025-09-18T01:56:16.545+09:00 level=INFO msg="migration up completed" component=migrator applied_count=1 duration_ms=15 dry_run=false
```

### Migration Rollback
```
time=2025-09-18T01:56:20.100+09:00 level=INFO msg="starting migration down" component=migrator target_version=0 dir=./migrations dry_run=false
time=2025-09-18T01:56:20.101+09:00 level=INFO msg="rolling back migration" component=migrator version=1 file="001_create_users.yaml"
time=2025-09-18T01:56:20.123+09:00 level=INFO msg="migration down completed" component=migrator rolled_back_count=1 duration_ms=23 dry_run=false
```

This approach provides **exactly what migration tools need**: clear progress visibility and performance insights without the overhead of enterprise monitoring systems.
