package common

import (
	"regexp"
	"strings"
	"testing"
)

func TestMasker_MaskString(t *testing.T) {
	masker := NewMasker()

	tests := []struct {
		name     string
		input    string
		contains string // What the result should contain
	}{
		{
			name:     "password in JSON",
			input:    `{"username": "admin", "password": "secret123"}`,
			contains: "***MASKED***",
		},
		{
			name:     "API key in JSON",
			input:    `{"api_key": "sk_test_1234567890abcdef"}`,
			contains: "***MASKED***",
		},
		{
			name:     "Bearer token",
			input:    `Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9`,
			contains: "***MASKED***",
		},
		{
			name:     "Basic auth",
			input:    `Authorization: Basic YWRtaW46cGFzc3dvcmQ=`,
			contains: "***MASKED***",
		},
		{
			name:     "client secret",
			input:    `"client_secret": "very_secret_value"`,
			contains: "***MASKED***",
		},
		{
			name:     "no sensitive data",
			input:    `{"username": "admin", "email": "admin@example.com"}`,
			contains: "admin@example.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := masker.MaskString(tt.input)
			if !strings.Contains(result, tt.contains) {
				t.Errorf("MaskString() result %q should contain %q", result, tt.contains)
			}
		})
	}
}

func TestMasker_MaskValue(t *testing.T) {
	masker := NewMasker()

	tests := []struct {
		name     string
		key      string
		value    interface{}
		expected interface{}
	}{
		{
			name:     "password key",
			key:      "password",
			value:    "secret123",
			expected: "***MASKED***",
		},
		{
			name:     "API key",
			key:      "api_key",
			value:    "sk_test_1234",
			expected: "***MASKED***",
		},
		{
			name:     "token key",
			key:      "token",
			value:    "jwt_token_here",
			expected: "***MASKED***",
		},
		{
			name:     "authorization key",
			key:      "authorization",
			value:    "Bearer token123",
			expected: "***MASKED***",
		},
		{
			name:     "normal key",
			key:      "username",
			value:    "admin",
			expected: "admin",
		},
		{
			name:     "case insensitive password",
			key:      "PASSWORD",
			value:    "secret123",
			expected: "***MASKED***",
		},
		{
			name:     "case insensitive API key",
			key:      "API_KEY",
			value:    "sk_test_1234",
			expected: "***MASKED***",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := masker.MaskValue(tt.key, tt.value)
			if result != tt.expected {
				t.Errorf("MaskValue(%q, %v) = %v, want %v", tt.key, tt.value, result, tt.expected)
			}
		})
	}
}

func TestMasker_MaskKeyValuePairs(t *testing.T) {
	masker := NewMasker()

	input := []any{"username", "admin", "password", "secret123", "api_key", "sk_test_1234", "email", "admin@example.com"}
	expected := []any{"username", "admin", "password", "***MASKED***", "api_key", "***MASKED***", "email", "admin@example.com"}

	result := masker.MaskKeyValuePairs(input...)

	if len(result) != len(expected) {
		t.Errorf("MaskKeyValuePairs() returned %d items, want %d", len(result), len(expected))
		return
	}

	for i, v := range result {
		if v != expected[i] {
			t.Errorf("MaskKeyValuePairs()[%d] = %v, want %v", i, v, expected[i])
		}
	}
}

func TestMasker_Disabled(t *testing.T) {
	masker := NewMasker()
	masker.SetEnabled(false)

	input := `{"password": "secret123", "api_key": "sk_test_1234"}`
	result := masker.MaskString(input)

	if result != input {
		t.Errorf("Disabled masker should return original string, got %q", result)
	}
}

func TestMasker_CustomPatterns(t *testing.T) {
	customPattern := SensitivePattern{
		Name:        "custom_secret",
		Regex:       regexp.MustCompile(`(?i)(custom[_-]?secret)["'\s]*[:=]["'\s]*([^"',}\]\s]+)`),
		Replacement: `${1}":"***CUSTOM_MASKED***"`,
		Keys:        []string{"custom_secret"},
	}

	masker := NewMaskerWithPatterns([]SensitivePattern{customPattern})

	input := `{"custom_secret": "my_secret_value"}`
	result := masker.MaskString(input)

	if !strings.Contains(result, "***CUSTOM_MASKED***") {
		t.Errorf("Custom pattern masking failed. Result should contain ***CUSTOM_MASKED***, got %q", result)
	}

	// Test with key matching
	result2 := masker.MaskValue("custom_secret", "my_secret_value")
	if result2 != "***MASKED***" {
		t.Errorf("Custom pattern key masking failed. Got %v, want %q", result2, "***MASKED***")
	}
}

func TestLogger_MaskingIntegration(t *testing.T) {
	// Create a logger with masking enabled
	logger := NewLogger(LogLevelInfo)

	// Test that masking is enabled by default
	if !logger.IsMaskingEnabled() {
		t.Error("Masking should be enabled by default")
	}

	// Test disabling masking
	logger.EnableMasking(false)
	if logger.IsMaskingEnabled() {
		t.Error("Masking should be disabled after calling EnableMasking(false)")
	}

	// Test enabling masking
	logger.EnableMasking(true)
	if !logger.IsMaskingEnabled() {
		t.Error("Masking should be enabled after calling EnableMasking(true)")
	}

	// Test that context methods preserve masking
	contextLogger := logger.WithComponent("test")
	if !contextLogger.IsMaskingEnabled() {
		t.Error("Context logger should preserve masking settings")
	}

	if contextLogger.GetMasker() != logger.GetMasker() {
		t.Error("Context logger should share the same masker instance")
	}
}

func TestGlobalMasking(t *testing.T) {
	// Test global masking functions
	originalState := IsMaskingEnabled()
	defer EnableMasking(originalState) // Restore original state

	// Test enabling global masking
	EnableMasking(true)
	if !IsMaskingEnabled() {
		t.Error("Global masking should be enabled")
	}

	// Test masking with global masker
	input := "password=secret123"
	masked := MaskSensitiveData(input)
	if masked == input {
		t.Error("Global masker should have masked the password")
	}

	// Test disabling global masking
	EnableMasking(false)
	if IsMaskingEnabled() {
		t.Error("Global masking should be disabled")
	}

	masked2 := MaskSensitiveData(input)
	if masked2 != input {
		t.Error("Disabled global masker should return original input")
	}
}

func TestSensitivePattern_Coverage(t *testing.T) {
	masker := NewMasker()

	// Test various sensitive patterns are covered
	testCases := []struct {
		input    string
		contains string
	}{
		{`"password": "secret"`, "***MASKED***"},
		{`"passwd": "secret"`, "***MASKED***"},
		{`"pwd": "secret"`, "***MASKED***"},
		{`"api_key": "secret"`, "***MASKED***"},
		{`"apikey": "secret"`, "***MASKED***"},
		{`"api-key": "secret"`, "***MASKED***"},
		{`"token": "secret"`, "***MASKED***"},
		{`"access_token": "secret"`, "***MASKED***"},
		{`"auth_token": "secret"`, "***MASKED***"},
		{`"authorization": "secret"`, "***MASKED***"},
		{`"secret": "secret"`, "***MASKED***"},
		{`"client_secret": "secret"`, "***MASKED***"},
		{`"client-secret": "secret"`, "***MASKED***"},
		{"Bearer token123", "Bearer ***MASKED***"},
		{"Basic base64string", "Basic ***MASKED***"},
	}

	for _, tc := range testCases {
		t.Run(tc.input, func(t *testing.T) {
			result := masker.MaskString(tc.input)
			if !strings.Contains(result, tc.contains) {
				t.Errorf("Expected %q to contain %q, got %q", tc.input, tc.contains, result)
			}
		})
	}
}
