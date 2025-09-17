package common

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"
)

func TestNewLogger(t *testing.T) {
	tests := []struct {
		name     string
		level    LogLevel
		expected slog.Level
	}{
		{"error level", LogLevelError, slog.LevelError},
		{"warn level", LogLevelWarn, slog.LevelWarn},
		{"info level", LogLevelInfo, slog.LevelInfo},
		{"debug level", LogLevelDebug, slog.LevelDebug},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := NewLogger(tt.level)
			if logger == nil {
				t.Fatal("expected logger, got nil")
			}
			if logger.Logger == nil {
				t.Fatal("expected slog.Logger, got nil")
			}
		})
	}
}

func TestNewJSONLogger(t *testing.T) {
	logger := NewJSONLogger(LogLevelInfo)
	if logger == nil {
		t.Fatal("expected logger, got nil")
	}
	if logger.Logger == nil {
		t.Fatal("expected slog.Logger, got nil")
	}
}

func TestLoggerWithContext(t *testing.T) {
	logger := NewLogger(LogLevelInfo)

	componentLogger := logger.WithComponent("test-component")
	if componentLogger == nil {
		t.Fatal("expected logger with component, got nil")
	}

	versionLogger := logger.WithVersion(123)
	if versionLogger == nil {
		t.Fatal("expected logger with version, got nil")
	}

	authLogger := logger.WithAuth("test-auth")
	if authLogger == nil {
		t.Fatal("expected logger with auth, got nil")
	}

	storeLogger := logger.WithStore("test-store")
	if storeLogger == nil {
		t.Fatal("expected logger with store, got nil")
	}

	requestLogger := logger.WithRequest("GET", "http://example.com")
	if requestLogger == nil {
		t.Fatal("expected logger with request, got nil")
	}
}

func TestGlobalLogger(t *testing.T) {
	// Test default logger
	defaultLogger := GetLogger()
	if defaultLogger == nil {
		t.Fatal("expected default logger, got nil")
	}

	// Test setting custom logger
	customLogger := NewLogger(LogLevelDebug)
	SetDefaultLogger(customLogger)

	retrievedLogger := GetLogger()
	if retrievedLogger != customLogger {
		t.Fatal("expected custom logger to be set as default")
	}

	// Reset to default for other tests
	SetDefaultLogger(NewLogger(LogLevelInfo))
}

func TestLogFunctions(t *testing.T) {
	var buf bytes.Buffer

	// Create a logger that writes to our buffer
	opts := &slog.HandlerOptions{Level: slog.LevelDebug}
	handler := slog.NewTextHandler(&buf, opts)
	logger := &Logger{Logger: slog.New(handler)}
	SetDefaultLogger(logger)

	// Test log functions
	LogInfo("test info message", "key", "value")
	LogDebug("test debug message", "debug_key", "debug_value")
	LogWarn("test warn message", "warn_key", "warn_value")
	LogError("test error message", nil, "error_key", "error_value")

	output := buf.String()

	if !strings.Contains(output, "test info message") {
		t.Error("expected info message in output")
	}
	if !strings.Contains(output, "test debug message") {
		t.Error("expected debug message in output")
	}
	if !strings.Contains(output, "test warn message") {
		t.Error("expected warn message in output")
	}
	if !strings.Contains(output, "test error message") {
		t.Error("expected error message in output")
	}
}
