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

// Logger provides a centralized logging interface for apimigrate
type Logger struct {
	*slog.Logger
}

// NewLogger creates a new structured logger with the specified level
func NewLogger(level LogLevel) *Logger {
	var slogLevel slog.Level
	switch level {
	case LogLevelError:
		slogLevel = slog.LevelError
	case LogLevelWarn:
		slogLevel = slog.LevelWarn
	case LogLevelInfo:
		slogLevel = slog.LevelInfo
	case LogLevelDebug:
		slogLevel = slog.LevelDebug
	default:
		slogLevel = slog.LevelInfo
	}

	opts := &slog.HandlerOptions{
		Level: slogLevel,
	}

	handler := slog.NewTextHandler(os.Stdout, opts)
	logger := slog.New(handler)

	return &Logger{Logger: logger}
}

// NewJSONLogger creates a structured logger with JSON output
func NewJSONLogger(level LogLevel) *Logger {
	var slogLevel slog.Level
	switch level {
	case LogLevelError:
		slogLevel = slog.LevelError
	case LogLevelWarn:
		slogLevel = slog.LevelWarn
	case LogLevelInfo:
		slogLevel = slog.LevelInfo
	case LogLevelDebug:
		slogLevel = slog.LevelDebug
	default:
		slogLevel = slog.LevelInfo
	}

	opts := &slog.HandlerOptions{
		Level: slogLevel,
	}

	handler := slog.NewJSONHandler(os.Stdout, opts)
	logger := slog.New(handler)

	return &Logger{Logger: logger}
}

// WithComponent returns a logger with component context
func (l *Logger) WithComponent(component string) *Logger {
	return &Logger{Logger: l.Logger.With("component", component)}
}

// WithVersion returns a logger with migration version context
func (l *Logger) WithVersion(version int) *Logger {
	return &Logger{Logger: l.Logger.With("version", version)}
}

// WithAuth returns a logger with authentication context
func (l *Logger) WithAuth(authName string) *Logger {
	return &Logger{Logger: l.Logger.With("auth", authName)}
}

// WithStore returns a logger with store context
func (l *Logger) WithStore(storeType string) *Logger {
	return &Logger{Logger: l.Logger.With("store", storeType)}
}

// WithRequest returns a logger with HTTP request context
func (l *Logger) WithRequest(method, url string) *Logger {
	return &Logger{Logger: l.Logger.With("method", method, "url", url)}
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
