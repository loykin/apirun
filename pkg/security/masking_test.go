package security

import (
	"strings"
	"testing"
)

func TestMasker_MaskValue(t *testing.T) {
	masker := NewMasker()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "very short value",
			input:    "abc",
			expected: "********",
		},
		{
			name:     "medium value",
			input:    "secret123",
			expected: "secr*****",
		},
		{
			name:     "long value",
			input:    "very_long_secret_token_12345",
			expected: "very************************",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := masker.MaskValue(tt.input)
			if result != tt.expected {
				t.Errorf("MaskValue(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestMasker_MaskLogKeyVals(t *testing.T) {
	masker := NewMasker()

	tests := []struct {
		name     string
		input    []interface{}
		expected []interface{}
	}{
		{
			name:     "empty slice",
			input:    []interface{}{},
			expected: []interface{}{},
		},
		{
			name: "password field",
			input: []interface{}{
				"username", "john",
				"password", "secret123",
				"email", "john@example.com",
			},
			expected: []interface{}{
				"username", "john",
				"password", "secr*****",
				"email", "john@example.com",
			},
		},
		{
			name: "API key field",
			input: []interface{}{
				"api_key", "sk-1234567890abcdef",
				"service", "api",
			},
			expected: []interface{}{
				"api_key", "sk-1***************",
				"service", "api",
			},
		},
		{
			name: "bearer token in value",
			input: []interface{}{
				"authorization", "Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9",
				"method", "GET",
			},
			expected: []interface{}{
				"authorization", "Bear***************************************",
				"method", "GET",
			},
		},
		{
			name: "non-string sensitive value",
			input: []interface{}{
				"token", 12345,
				"count", 10,
			},
			expected: []interface{}{
				"token", "********",
				"count", 10,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := masker.MaskLogKeyVals(tt.input)
			if len(result) != len(tt.expected) {
				t.Fatalf("Length mismatch: got %d, want %d", len(result), len(tt.expected))
			}
			for i, v := range result {
				if v != tt.expected[i] {
					t.Errorf("Index %d: got %v, want %v", i, v, tt.expected[i])
				}
			}
		})
	}
}

func TestMasker_MaskHeaders(t *testing.T) {
	masker := NewMasker()

	headers := map[string][]string{
		"Content-Type":  {"application/json"},
		"Authorization": {"Bearer token123"},
		"X-API-Key":     {"api_key_123456"},
		"User-Agent":    {"apirun/1.0"},
	}

	result := masker.MaskHeaders(headers)

	// Authorization should be masked
	if result["Authorization"][0] != "Bear***********" {
		t.Errorf("Authorization not masked properly: %s", result["Authorization"][0])
	}

	// X-API-Key should be masked
	if result["X-API-Key"][0] != "api_**********" {
		t.Errorf("X-API-Key not masked properly: %s", result["X-API-Key"][0])
	}

	// Content-Type should not be masked
	if result["Content-Type"][0] != "application/json" {
		t.Errorf("Content-Type should not be masked: %s", result["Content-Type"][0])
	}

	// User-Agent should not be masked
	if result["User-Agent"][0] != "apirun/1.0" {
		t.Errorf("User-Agent should not be masked: %s", result["User-Agent"][0])
	}
}

func TestMasker_MaskURL(t *testing.T) {
	masker := NewMasker()

	tests := []struct {
		name     string
		input    string
		contains string // Check if result contains this substring
	}{
		{
			name:     "postgres URL with password",
			input:    "postgres://user:secret123@localhost:5432/db",
			contains: "****",
		},
		{
			name:     "mysql URL with password",
			input:    "mysql://admin:password@localhost:3306/mydb",
			contains: "****",
		},
		{
			name:     "regular URL",
			input:    "https://api.example.com/v1/users",
			contains: "api", // Should contain some part of the original
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := masker.MaskURL(tt.input)
			if !strings.Contains(result, tt.contains) {
				t.Errorf("MaskURL(%q) = %q, should contain %q", tt.input, result, tt.contains)
			}
		})
	}
}

func TestIsSensitiveKey(t *testing.T) {
	masker := NewMasker()

	sensitiveKeys := []string{
		"password", "Password", "PASSWORD",
		"secret", "api_key", "token",
		"authorization", "credential",
		"private_key", "jwt", "oauth",
	}

	normalKeys := []string{
		"username", "email", "name",
		"url", "method", "status",
		"count", "id", "timestamp",
	}

	for _, key := range sensitiveKeys {
		if !masker.isSensitiveKey(key) {
			t.Errorf("Key %q should be considered sensitive", key)
		}
	}

	for _, key := range normalKeys {
		if masker.isSensitiveKey(key) {
			t.Errorf("Key %q should not be considered sensitive", key)
		}
	}
}

func TestIsSensitiveHeader(t *testing.T) {
	masker := NewMasker()

	sensitiveHeaders := []string{
		"Authorization", "authorization", "AUTHORIZATION",
		"X-API-Key", "x-api-key", "Cookie",
		"Set-Cookie", "X-Auth-Token",
	}

	normalHeaders := []string{
		"Content-Type", "User-Agent", "Accept",
		"Content-Length", "Host", "Referer",
	}

	for _, header := range sensitiveHeaders {
		if !masker.isSensitiveHeader(header) {
			t.Errorf("Header %q should be considered sensitive", header)
		}
	}

	for _, header := range normalHeaders {
		if masker.isSensitiveHeader(header) {
			t.Errorf("Header %q should not be considered sensitive", header)
		}
	}
}

func TestContainsSensitivePattern(t *testing.T) {
	masker := NewMasker()

	sensitiveValues := []string{
		"Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9",
		"Basic dXNlcjpwYXNzd29yZA==",
		"postgres://user:pass@localhost/db",
		"abcdef1234567890abcdef1234567890", // 32+ char API key
	}

	normalValues := []string{
		"GET", "POST", "application/json",
		"user@example.com", "localhost:8080",
		"short", "abc123", // Too short for API key pattern
	}

	for _, value := range sensitiveValues {
		if !masker.containsSensitivePattern(value) {
			t.Errorf("Value %q should match sensitive pattern", value)
		}
	}

	for _, value := range normalValues {
		if masker.containsSensitivePattern(value) {
			t.Errorf("Value %q should not match sensitive pattern", value)
		}
	}
}

func TestDefaultMaskerFunctions(t *testing.T) {
	// Test that the convenience functions work
	value := "secret123"
	masked := MaskValue(value)
	if masked == value {
		t.Error("Default MaskValue should mask the value")
	}

	keyvals := []interface{}{"password", "secret"}
	maskedKeyvals := MaskLogKeyVals(keyvals)
	if maskedKeyvals[1] == "secret" {
		t.Error("Default MaskLogKeyVals should mask sensitive values")
	}

	headers := map[string][]string{"Authorization": {"Bearer token"}}
	maskedHeaders := MaskHeaders(headers)
	if maskedHeaders["Authorization"][0] == "Bearer token" {
		t.Error("Default MaskHeaders should mask sensitive headers")
	}

	url := "postgres://user:pass@localhost/db"
	maskedURL := MaskURL(url)
	if maskedURL == url {
		t.Error("Default MaskURL should mask sensitive URLs")
	}
}
