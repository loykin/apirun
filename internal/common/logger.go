package common

import (
	"log/slog"
	"os"
)

// LogLevel represents logging verbosity levels
type LogLevel int

const (
	LogLevelError LogLevel = iota
	LogLevelWarn
	LogLevelInfo
	LogLevelDebug
)

// String returns the string representation of the log level
func (l LogLevel) String() string {
	switch l {
	case LogLevelError:
		return "error"
	case LogLevelWarn:
		return "warn"
	case LogLevelInfo:
		return "info"
	case LogLevelDebug:
		return "debug"
	default:
		return "info"
	}
}

// ToSlogLevel converts LogLevel to slog.Level
func (l LogLevel) ToSlogLevel() slog.Level {
	switch l {
	case LogLevelError:
		return slog.LevelError
	case LogLevelWarn:
		return slog.LevelWarn
	case LogLevelInfo:
		return slog.LevelInfo
	case LogLevelDebug:
		return slog.LevelDebug
	default:
		return slog.LevelInfo
	}
}

// Logger provides a centralized logging interface for apimigrate
type Logger struct {
	*slog.Logger
	level LogLevel
}

// NewLogger creates a new structured logger with the specified level
func NewLogger(level LogLevel) *Logger {
	opts := &slog.HandlerOptions{
		Level: level.ToSlogLevel(),
	}

	handler := slog.NewTextHandler(os.Stdout, opts)
	logger := slog.New(handler)

	return &Logger{
		Logger: logger,
		level:  level,
	}
}

// NewJSONLogger creates a structured logger with JSON output
func NewJSONLogger(level LogLevel) *Logger {
	opts := &slog.HandlerOptions{
		Level: level.ToSlogLevel(),
	}

	handler := slog.NewJSONHandler(os.Stdout, opts)
	logger := slog.New(handler)

	return &Logger{
		Logger: logger,
		level:  level,
	}
}

// Level returns the current log level
func (l *Logger) Level() LogLevel {
	return l.level
}

// WithComponent returns a logger with component context
func (l *Logger) WithComponent(component string) *Logger {
	return &Logger{
		Logger: l.Logger.With("component", component),
		level:  l.level,
	}
}

// WithVersion returns a logger with migration version context
func (l *Logger) WithVersion(version int) *Logger {
	return &Logger{
		Logger: l.Logger.With("version", version),
		level:  l.level,
	}
}

// WithAuth returns a logger with authentication context
func (l *Logger) WithAuth(authName string) *Logger {
	return &Logger{
		Logger: l.Logger.With("auth", authName),
		level:  l.level,
	}
}

// WithStore returns a logger with store context
func (l *Logger) WithStore(storeType string) *Logger {
	return &Logger{
		Logger: l.Logger.With("store", storeType),
		level:  l.level,
	}
}

// WithRequest returns a logger with HTTP request context
func (l *Logger) WithRequest(method, url string) *Logger {
	return &Logger{
		Logger: l.Logger.With("method", method, "url", url),
		level:  l.level,
	}
}

// Global default logger instance
var defaultLogger = NewLogger(LogLevelInfo)

// SetDefaultLogger sets the global default logger
func SetDefaultLogger(logger *Logger) {
	defaultLogger = logger
}

// GetLogger returns the default logger
func GetLogger() *Logger {
	return defaultLogger
}

// LogError logs an error with context
func LogError(msg string, err error, attrs ...any) {
	args := append([]any{"error", err}, attrs...)
	defaultLogger.Error(msg, args...)
}

// LogInfo logs informational message
func LogInfo(msg string, attrs ...any) {
	defaultLogger.Info(msg, attrs...)
}

// LogDebug logs debug message
func LogDebug(msg string, attrs ...any) {
	defaultLogger.Debug(msg, attrs...)
}

// LogWarn logs warning message
func LogWarn(msg string, attrs ...any) {
	defaultLogger.Warn(msg, attrs...)
}
