# Color Logging Demo

This example demonstrates the colorized logging feature of apirun, which improves readability for CLI usage while
maintaining compatibility with embedded usage.

## Features

### 🎨 **Automatic Color Detection**

- Colors are automatically enabled when running in a terminal
- Automatically disabled when output is redirected to files or pipes
- Can be explicitly controlled via configuration

### 🌈 **Color Scheme**

- **DEBUG**: Gray text
- **INFO**: Green level indicator
- **WARN**: Yellow level indicator
- **ERROR**: Red level indicator
- **Components**: Cyan highlighting
- **Values**: Different colors for different data types
- **Success indicators**: Green
- **Error messages**: Red

### 📋 **Log Format Comparison**

#### Plain Text (default)

```
time=2025-09-18T02:02:58.660+09:00 level=INFO msg="Migration started" version=1 file=001_create_users.yaml
```

#### JSON Format

```
{"time":"2025-09-18T02:02:58.764+09:00","level":"INFO","msg":"Migration started","version":1,"file":"001_create_users.yaml"}
```

#### Color Format

```
2025-09-18T02:02:58+09:00 [INFO ] Migration started version=1 file="001_create_users.yaml"
```

*(with actual colors in terminals)*

## Configuration

### Basic Color Logging

```yaml
logging:
  format: color
  level: info
  mask_sensitive: true
```

### Explicit Color Control

```yaml
logging:
  format: text
  color: true    # Force colors even with text format
  level: info
```

### Auto-Detection (Recommended)

```yaml
logging:
  format: color  # Will auto-detect terminal capability
  level: info
```

## When to Use Colors

### ✅ **Perfect for CLI usage:**

- Interactive terminal sessions
- Development and debugging
- Manual migration runs
- Better visual distinction between log levels

### ❌ **Not recommended for:**

- Log files and aggregation systems
- Embedded library usage
- CI/CD pipelines (unless explicitly desired)
- Non-interactive environments

## Running the Demo

```bash
go run ./examples/color_demo
```

## Integration Examples

### CLI Tool with Colors

```go
// Automatically detects terminal and enables colors
logger := apirun.NewColorLogger(apirun.LogLevelInfo)

// Or via configuration
migrator := apirun.Migrator{
Dir: "./migrations",
}
// Colors configured via config.yaml format: color
```

### Embedded Usage (No Colors)

```go
// Use plain text for embedded/library usage
logger := apirun.NewLogger(apirun.LogLevelInfo)

// Or JSON for log aggregation  
logger := apirun.NewJSONLogger(apirun.LogLevelInfo)
```

### Smart Detection

```go
// The color handler automatically detects:
// - Is this a terminal? → Use colors
// - Is output redirected? → No colors  
// - Is this Windows? → No colors (by default)
logger := apirun.NewColorLogger(apirun.LogLevelInfo)
```

## Color Examples

When running in a terminal, you'll see:

- 🟢 **[INFO ]** in green for successful operations
- 🟡 **[WARN ]** in yellow for warnings
- 🔴 **[ERROR]** in red for failures
- 🔵 **[DEBUG]** in gray for debug info
- 🌀 **Component names** in cyan
- 📊 **Numbers** in magenta
- ✅ **Success values** in green
- ❌ **Error messages** in red

This makes it much easier to quickly scan logs and identify issues during interactive migration sessions!

## Smart Color Detection

The system automatically:

1. **Detects terminals** - Colors enabled in interactive terminals
2. **Detects redirects** - Colors disabled when piping to files
3. **Respects configuration** - Manual override via config files
4. **Platform aware** - Considers OS capabilities

Perfect for CLI tools that need both **interactive usability** and **embedded compatibility**!
