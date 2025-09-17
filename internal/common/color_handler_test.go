package common

import (
	"bytes"
	"context"
	"log/slog"
	"os"
	"strings"
	"testing"
	"time"
)

func TestNewColorHandler(t *testing.T) {
	var buf bytes.Buffer
	handler := NewColorHandler(&buf, nil)

	if handler == nil {
		t.Fatal("NewColorHandler returned nil")
	}

	if handler.writer != &buf {
		t.Error("Writer not set correctly")
	}

	if handler.masker == nil {
		t.Error("Masker not initialized")
	}
}

func TestColorHandler_Enabled(t *testing.T) {
	var buf bytes.Buffer

	tests := []struct {
		name    string
		level   slog.Level
		opts    *slog.HandlerOptions
		enabled bool
	}{
		{
			name:    "default level (info)",
			level:   slog.LevelInfo,
			opts:    nil,
			enabled: true,
		},
		{
			name:    "debug level with info handler",
			level:   slog.LevelDebug,
			opts:    nil,
			enabled: false,
		},
		{
			name:    "error level",
			level:   slog.LevelError,
			opts:    nil,
			enabled: true,
		},
		{
			name:    "debug handler with debug level",
			level:   slog.LevelDebug,
			opts:    &slog.HandlerOptions{Level: slog.LevelDebug},
			enabled: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := NewColorHandler(&buf, tt.opts)
			ctx := context.Background()

			enabled := handler.Enabled(ctx, tt.level)
			if enabled != tt.enabled {
				t.Errorf("Expected enabled=%t, got %t", tt.enabled, enabled)
			}
		})
	}
}

func TestColorHandler_Handle(t *testing.T) {
	var buf bytes.Buffer
	handler := NewColorHandler(&buf, nil)
	handler.useColor = false // Disable colors for testing

	ctx := context.Background()
	timestamp := time.Date(2025, 9, 18, 10, 30, 45, 0, time.UTC)

	record := slog.NewRecord(timestamp, slog.LevelInfo, "test message", 0)
	record.Add("testkey1", "value1")
	record.Add("testkey2", 42)
	record.Add("testkey3", true)

	err := handler.Handle(ctx, record)
	if err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}

	output := buf.String()

	// Check if log contains expected components
	if !strings.Contains(output, "2025-09-18T10:30:45Z") {
		t.Error("Output missing timestamp")
	}
	if !strings.Contains(output, "[INFO ]") {
		t.Error("Output missing formatted level")
	}
	if !strings.Contains(output, "test message") {
		t.Error("Output missing message")
	}
	if !strings.Contains(output, "testkey1=\"value1\"") {
		t.Error("Output missing string attribute")
	}
	if !strings.Contains(output, "testkey2=42") {
		t.Error("Output missing int attribute")
	}
	if !strings.Contains(output, "testkey3=true") {
		t.Error("Output missing bool attribute")
	}
}

func TestColorHandler_FormatLevel(t *testing.T) {
	var buf bytes.Buffer
	handler := NewColorHandler(&buf, nil)
	handler.useColor = false // Test without colors

	tests := []struct {
		level    slog.Level
		expected string
	}{
		{slog.LevelDebug, "[DEBUG]"},
		{slog.LevelInfo, "[INFO ]"},
		{slog.LevelWarn, "[WARN ]"},
		{slog.LevelError, "[ERROR]"},
	}

	for _, tt := range tests {
		t.Run(tt.level.String(), func(t *testing.T) {
			result := handler.formatLevel(tt.level)
			if result != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestColorHandler_ColorizeWithColorsDisabled(t *testing.T) {
	var buf bytes.Buffer
	handler := NewColorHandler(&buf, nil)
	handler.useColor = false

	result := handler.colorize(Red, "test")
	expected := "test"

	if result != expected {
		t.Errorf("Expected %q, got %q", expected, result)
	}
}

func TestColorHandler_ColorizeWithColorsEnabled(t *testing.T) {
	var buf bytes.Buffer
	handler := NewColorHandler(&buf, nil)
	handler.useColor = true

	result := handler.colorize(Red, "test")
	expected := Red + "test" + Reset

	if result != expected {
		t.Errorf("Expected %q, got %q", expected, result)
	}
}

func TestColorHandler_IsErrorLike(t *testing.T) {
	var buf bytes.Buffer
	handler := NewColorHandler(&buf, nil)

	tests := []struct {
		input    string
		expected bool
	}{
		{"error occurred", true},
		{"Error message", true},
		{"failed to connect", true},
		{"operation failed", true},
		{"exception thrown", true},
		{"success", false},
		{"completed", false},
		{"normal message", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := handler.isErrorLike(tt.input)
			if result != tt.expected {
				t.Errorf("For input %q, expected %t, got %t", tt.input, tt.expected, result)
			}
		})
	}
}

func TestColorHandler_IsSuccessLike(t *testing.T) {
	var buf bytes.Buffer
	handler := NewColorHandler(&buf, nil)

	tests := []struct {
		input    string
		expected bool
	}{
		{"success", true},
		{"Success!", true},
		{"completed successfully", true},
		{"operation complete", true},
		{"ok", true},
		{"applied", true},
		{"error", false},
		{"failed", false},
		{"normal message", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := handler.isSuccessLike(tt.input)
			if result != tt.expected {
				t.Errorf("For input %q, expected %t, got %t", tt.input, tt.expected, result)
			}
		})
	}
}

func TestColorHandler_WithAttrs(t *testing.T) {
	var buf bytes.Buffer
	handler := NewColorHandler(&buf, nil)

	attrs := []slog.Attr{
		slog.String("key1", "value1"),
		slog.Int("key2", 42),
	}

	newHandler := handler.WithAttrs(attrs)
	colorHandler, ok := newHandler.(*ColorHandler)
	if !ok {
		t.Fatal("WithAttrs did not return a ColorHandler")
	}

	if len(colorHandler.attrs) != 2 {
		t.Errorf("Expected 2 attributes, got %d", len(colorHandler.attrs))
	}

	if colorHandler.attrs[0].Key != "key1" {
		t.Errorf("Expected first attribute key to be 'key1', got %q", colorHandler.attrs[0].Key)
	}
}

func TestColorHandler_WithGroup(t *testing.T) {
	var buf bytes.Buffer
	handler := NewColorHandler(&buf, nil)

	newHandler := handler.WithGroup("test_component")
	colorHandler, ok := newHandler.(*ColorHandler)
	if !ok {
		t.Fatal("WithGroup did not return a ColorHandler")
	}

	if len(colorHandler.groups) != 1 {
		t.Errorf("Expected 1 group, got %d", len(colorHandler.groups))
	}

	if colorHandler.groups[0] != "test_component" {
		t.Errorf("Expected group to be 'test_component', got %q", colorHandler.groups[0])
	}
}

func TestColorHandler_MaskingIntegration(t *testing.T) {
	var buf bytes.Buffer
	handler := NewColorHandler(&buf, nil)
	handler.useColor = false // Disable colors for easier testing

	ctx := context.Background()
	timestamp := time.Date(2025, 9, 18, 10, 30, 45, 0, time.UTC)

	record := slog.NewRecord(timestamp, slog.LevelInfo, "authentication", 0)
	record.Add("username", "admin")
	record.Add("password", "secret123")
	record.Add("api_key", "sk_test_1234567890")

	err := handler.Handle(ctx, record)
	if err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}

	output := buf.String()

	// Check that sensitive data is masked
	if !strings.Contains(output, "username=\"admin\"") {
		t.Error("Username should not be masked")
	}
	if strings.Contains(output, "secret123") {
		t.Error("Password should be masked")
	}
	if strings.Contains(output, "sk_test_1234567890") {
		t.Error("API key should be masked")
	}
	if !strings.Contains(output, "***MASKED***") {
		t.Error("Output should contain masked values")
	}
}

func TestColorHandler_SetColorEnabled(t *testing.T) {
	var buf bytes.Buffer
	handler := NewColorHandler(&buf, nil)

	// Test enabling colors
	handler.SetColorEnabled(true)
	if !handler.useColor {
		t.Error("SetColorEnabled(true) should enable colors")
	}

	// Test disabling colors
	handler.SetColorEnabled(false)
	if handler.useColor {
		t.Error("SetColorEnabled(false) should disable colors")
	}
}

func TestColorHandler_HandleWithGroup(t *testing.T) {
	var buf bytes.Buffer
	handler := NewColorHandler(&buf, nil)
	handler.useColor = false // Disable colors for testing

	// Add a group
	handlerWithGroup := handler.WithGroup("migrator")

	ctx := context.Background()
	timestamp := time.Date(2025, 9, 18, 10, 30, 45, 0, time.UTC)

	record := slog.NewRecord(timestamp, slog.LevelInfo, "test message", 0)

	err := handlerWithGroup.Handle(ctx, record)
	if err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}

	output := buf.String()

	// Check if component is included
	if !strings.Contains(output, "[migrator]") {
		t.Error("Output should contain component group")
	}
}

func TestShouldUseColor(t *testing.T) {
	// Test with bytes.Buffer (not a terminal)
	var buf bytes.Buffer
	if shouldUseColor(&buf) {
		t.Error("shouldUseColor should return false for bytes.Buffer")
	}

	// Test with os.Stdout (might be a terminal, depends on test environment)
	// We can't reliably test this as it depends on how tests are run
	result := shouldUseColor(os.Stdout)
	// Just verify it returns a boolean without panicking
	_ = result
}

func TestFormatValue(t *testing.T) {
	var buf bytes.Buffer
	handler := NewColorHandler(&buf, nil)
	handler.useColor = false // Test without colors

	tests := []struct {
		name     string
		value    slog.Value
		contains string
	}{
		{
			name:     "string value",
			value:    slog.StringValue("test"),
			contains: "\"test\"",
		},
		{
			name:     "int value",
			value:    slog.Int64Value(42),
			contains: "42",
		},
		{
			name:     "bool true",
			value:    slog.BoolValue(true),
			contains: "true",
		},
		{
			name:     "bool false",
			value:    slog.BoolValue(false),
			contains: "false",
		},
		{
			name:     "float value",
			value:    slog.Float64Value(3.14),
			contains: "3.14",
		},
		{
			name:     "duration value",
			value:    slog.DurationValue(5 * time.Second),
			contains: "5s",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := handler.formatValue(tt.value)
			if !strings.Contains(result, tt.contains) {
				t.Errorf("Expected result to contain %q, got %q", tt.contains, result)
			}
		})
	}
}
