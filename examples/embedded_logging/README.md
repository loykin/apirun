# Logging Example

This example demonstrates the structured logging capabilities added to apirun using the public API.

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

- Use `apirun.LogLevelDebug` to see all logging details
- Use `apirun.LogLevelInfo` for production-level logging
- Use `apirun.LogLevelError` for error-only logging

## Configuration

The example initializes logging at debug level to demonstrate all available log messages:

```go
logger := apirun.NewLogger(apirun.LogLevelDebug)
apirun.SetDefaultLogger(logger)
```

For production use, initialize with info level:

```go
logger := apirun.NewLogger(apirun.LogLevelInfo)
apirun.SetDefaultLogger(logger)
```

For JSON output (suitable for log aggregation):

```go
logger := apirun.NewJSONLogger(apirun.LogLevelInfo)
apirun.SetDefaultLogger(logger)
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

This example demonstrates the proper way to use apirun's logging functionality:

- `apirun.NewLogger()` - Create text logger
- `apirun.NewJSONLogger()` - Create JSON logger
- `apirun.SetDefaultLogger()` - Set global logger
- `apirun.GetLogger()` - Get current global logger

This approach ensures proper encapsulation and avoids importing internal packages.
