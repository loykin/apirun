# Logging Example

This example demonstrates the structured logging capabilities added to apimigrate using the public API.

## Features Demonstrated

- Initialization of structured logging with configurable levels
- Migration execution with detailed logging
- Error handling with structured context
- Success logging with result summaries
- **Proper use of public API** (no `internal` package imports)

## Running the Example

```bash
go run ./examples/embedded_logging
```

## Expected Output

The example will produce structured log output showing:

- Application lifecycle events
- Migration discovery and execution
- Database operations (if applicable)
- HTTP requests and responses
- Error contexts and success confirmations

## Log Levels

- Use `apimigrate.LogLevelDebug` to see all logging details
- Use `apimigrate.LogLevelInfo` for production-level logging
- Use `apimigrate.LogLevelError` for error-only logging

## Configuration

The example initializes logging at debug level to demonstrate all available log messages:

```go
logger := apimigrate.NewLogger(apimigrate.LogLevelDebug)
apimigrate.SetDefaultLogger(logger)
```

For production use, initialize with info level:

```go
logger := apimigrate.NewLogger(apimigrate.LogLevelInfo)
apimigrate.SetDefaultLogger(logger)
```

For JSON output (suitable for log aggregation):

```go
logger := apimigrate.NewJSONLogger(apimigrate.LogLevelInfo)
apimigrate.SetDefaultLogger(logger)
```

## Configuration File Support

You can also configure logging via `config.yaml`:

```yaml
logging:
  level: debug  # error, warn, info, debug
  format: json  # text, json
```

The CLI `--verbose` flag will override the config file setting to debug level.

## Public API

This example demonstrates the proper way to use apimigrate's logging functionality:

- `apimigrate.NewLogger()` - Create text logger
- `apimigrate.NewJSONLogger()` - Create JSON logger
- `apimigrate.SetDefaultLogger()` - Set global logger
- `apimigrate.GetLogger()` - Get current global logger

This approach ensures proper encapsulation and avoids importing internal packages.
