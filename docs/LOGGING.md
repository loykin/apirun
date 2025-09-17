# Logging Improvements - Implementation Summary

## Overview

This document summarizes the structured logging improvements made to the apimigrate codebase using Go's built-in `slog`
package.

## New Logging Framework

### Central Logging Configuration (`internal/common/logger.go`)

- **Structured Logging**: Implemented using Go's `slog` package for consistent, structured log output
- **Configurable Log Levels**: Support for Error, Warn, Info, and Debug levels
- **Multiple Output Formats**: Text and JSON handlers for different environments
- **Context-Aware Logging**: Component-specific loggers with contextual information
- **Global Logger Management**: Centralized logger configuration and access

### Key Features

- **Component Context**: `WithComponent(component)` adds component identification
- **Version Context**: `WithVersion(version)` adds migration version tracking
- **Auth Context**: `WithAuth(authName)` adds authentication provider context
- **Store Context**: `WithStore(storeType)` adds database store context
- **Request Context**: `WithRequest(method, url)` adds HTTP request context

## Logging Implementation Across Components

### Database Store Operations (`internal/store/`)

#### SQLite Store (`sqlite_store.go`)

- **Connection Logging**: Database connection establishment and configuration
- **Schema Logging**: Table creation and schema setup operations
- **Migration Version Logging**: Version application and status checking
- **Error Context**: Detailed error information with DSN and operation context

#### PostgreSQL Store (`postgresql_store.go`)

- **Connection Logging**: PostgreSQL connection establishment
- **Consistent Error Handling**: Aligned with SQLite store logging patterns

### Migration System (`internal/migration/migrator.go`)

- **Migration Start**: Logs migration initiation with target version and directory
- **File Discovery**: Logs discovered migration files and counts
- **Current Version**: Logs current migration state
- **Dry Run Support**: Specific logging for dry run mode operations
- **Authentication Flow**: Logs authentication setup and failures

### Authentication System (`internal/auth/registry.go`)

- **Provider Registration**: Logs auth provider type and configuration
- **Token Acquisition**: Logs successful and failed token acquisitions
- **Provider Validation**: Logs unsupported provider types and configuration errors

### HTTP Client Operations (`internal/httpc/httpc.go`)

- **Client Creation**: Logs HTTP client configuration
- **TLS Configuration**: Logs TLS settings when applied
- **Request Context**: Preparation for request/response logging

### Task Execution (`internal/task/up.go`)

- **Task Execution**: Logs task name, method, and URL
- **Request Processing**: Logs request rendering and HTTP details
- **Response Handling**: Logs status codes and response sizes
- **Error Context**: Detailed error information for failed requests

### Command Line Interface (`cmd/apimigrate/main.go`)

- **Logging Initialization**: Configures logging based on verbose flag
- **Configuration Loading**: Logs config file loading and processing
- **Application Lifecycle**: Logs application start with key parameters

## Usage Examples

### Basic Usage (Public API)

```go
// Initialize logger with appropriate level
logger := apimigrate.NewLogger(apimigrate.LogLevelInfo)
apimigrate.SetDefaultLogger(logger)

// Use structured logging
logger.Info("starting migration", "version", 123, "dir", "/migrations")
```

### JSON Logging for Production

```go
// Create JSON logger for log aggregation systems
logger := apimigrate.NewJSONLogger(apimigrate.LogLevelInfo)
apimigrate.SetDefaultLogger(logger)

// All internal components will now use structured JSON logging
```

### Integration with Migration System

```go
// The logging is automatically used by Migrator
m := apimigrate.Migrator{
Dir: "./migrations",
Env: env,
}

// Set logger before running migrations
logger := apimigrate.NewLogger(apimigrate.LogLevelDebug)
apimigrate.SetDefaultLogger(logger)

results, err := m.MigrateUp(ctx, 0)
// Detailed structured logs will be automatically generated
```

## Benefits

### For Development

- **Debugging**: Detailed context helps identify issues quickly
- **Performance Monitoring**: Request timing and response size tracking
- **State Tracking**: Migration version and status progression

### For Production

- **Structured Output**: JSON format for log aggregation systems
- **Configurable Verbosity**: Info level for production, Debug for troubleshooting
- **Error Context**: Rich error information for incident resolution

### For Operations

- **Component Identification**: Clear component boundaries in logs
- **Request Tracing**: HTTP request/response correlation
- **Database Operations**: Connection and transaction visibility

## Configuration

### Log Levels

- **Error**: Critical errors and failures only
- **Warn**: Warnings and recoverable issues
- **Info**: General application flow and status (default)
- **Debug**: Detailed debugging information

### Output Formats

- **Text**: Human-readable format for development (`apimigrate.NewLogger()`)
- **JSON**: Machine-readable format for production systems (`apimigrate.NewJSONLogger()`)

### Environment Integration

- **Verbose Flag**: CLI `--verbose` flag enables debug logging (overrides config)
- **Configuration File**: Set logging level and format in `config.yaml`
- **Programmatic Control**: Use public API to set log levels in code
- **Global Configuration**: Set once with `apimigrate.SetDefaultLogger()`

## Configuration File Support

### YAML Configuration (`config/config.yaml`)

```yaml
# Logging configuration
logging:
  # Log level: error, warn, info, debug (default: info)
  # Note: --verbose/-v flag overrides this to debug level
  level: info
  # Log format: text, json (default: text)
  # Use 'json' for production/log aggregation systems
  format: text
```

### Configuration Precedence

1. **CLI `--verbose` flag**: Overrides config to debug level
2. **Config file `logging.level`**: Sets base log level
3. **Default**: Info level if not specified

### Format Options

- **`text`**: Human-readable format for development and debugging
- **`json`**: Structured JSON format for production and log aggregation

### Examples

#### Development Configuration

```yaml
logging:
  level: debug
  format: text
```

#### Production Configuration

```yaml
logging:
  level: info
  format: json
```

#### Error-only Logging

```yaml
logging:
  level: error
  format: json
```

## Public API Reference

The apimigrate package provides a clean public API for logging configuration:

### Types and Constants

```go
// Log levels
const (
LogLevelError LogLevel = iota
LogLevelWarn
LogLevelInfo
LogLevelDebug
)
```

### Functions

```go
// Create loggers
func NewLogger(level LogLevel) *Logger
func NewJSONLogger(level LogLevel) *Logger

// Global logger management  
func SetDefaultLogger(logger *Logger)
func GetLogger() *Logger
```

### Best Practices

- Use `apimigrate.NewLogger()` for development/testing
- Use `apimigrate.NewJSONLogger()` for production deployments
- Set logger once at application startup with `SetDefaultLogger()`
- All internal apimigrate components will automatically use the configured logger

## Testing

### Unit Tests (`internal/common/logger_test.go`)

- **Logger Creation**: Tests for different log levels and formats
- **Context Methods**: Tests for component, version, auth, store, and request contexts
- **Global Logger**: Tests for default logger management
- **Output Validation**: Tests for log message content and structure

### Integration Testing

- **Store Operations**: Logging verified in database connection and migration operations
- **HTTP Requests**: Logging verified in task execution and auth flows
- **Error Scenarios**: Logging verified in failure cases and error conditions

## Future Enhancements

### Potential Additions

- **Request ID Correlation**: UUID correlation across request lifecycle
- **Metrics Integration**: Prometheus metrics based on log data
- **Log Sampling**: High-volume log sampling for performance
- **Audit Logging**: Security-focused audit trail logging

### Configuration Improvements

- **Config File Integration**: Log level and format in YAML configuration
- **Dynamic Log Levels**: Runtime log level adjustment
- **Log Rotation**: Built-in log file rotation support

## Compatibility

### Backward Compatibility

- **Existing Code**: No breaking changes to existing functionality
- **Test Suite**: All existing tests continue to pass
- **CLI Interface**: No changes to command-line interface

### Performance Impact

- **Minimal Overhead**: Structured logging with minimal performance impact
- **Conditional Logging**: Debug messages skipped when not enabled
- **Efficient Serialization**: Optimized JSON and text output

This logging implementation provides a solid foundation for debugging, monitoring, and operating the apimigrate system
while maintaining backward compatibility and performance.
