package security

import (
	"regexp"
	"strings"
)

// SensitiveFields contains patterns to identify sensitive information
var SensitiveFields = struct {
	// Field names that typically contain sensitive data
	FieldNames []string
	// Patterns to match sensitive values
	ValuePatterns []*regexp.Regexp
	// Header names that contain sensitive data
	HeaderNames []string
}{
	FieldNames: []string{
		"password", "passwd", "pwd",
		"secret", "token", "key", "auth", "authorization",
		"credential", "api_key", "apikey", "access_token",
		"refresh_token", "session", "cookie", "x-api-key",
		"bearer", "basic", "oauth", "jwt", "signature",
		"private_key", "public_key", "cert", "certificate",
		"dsn", "connection_string", "database_url",
	},
	ValuePatterns: []*regexp.Regexp{
		// Bearer tokens
		regexp.MustCompile(`(?i)bearer\s+[a-zA-Z0-9\-._~+/]+`),
		// Basic auth
		regexp.MustCompile(`(?i)basic\s+[a-zA-Z0-9+/=]+`),
		// JWT tokens (simplified pattern)
		regexp.MustCompile(`[a-zA-Z0-9\-_]+\.[a-zA-Z0-9\-_]+\.[a-zA-Z0-9\-_]+`),
		// API keys (common patterns)
		regexp.MustCompile(`[a-zA-Z0-9]{32,}`),
		// Database connection strings
		regexp.MustCompile(`(?i)(postgres|mysql|mongodb)://[^@]+:[^@]+@`),
	},
	HeaderNames: []string{
		"authorization", "x-api-key", "x-auth-token",
		"cookie", "set-cookie", "x-session-token",
		"x-access-token", "x-refresh-token",
	},
}

// Masker provides methods to mask sensitive information
type Masker struct {
	// MaskChar is the character used for masking
	MaskChar rune
	// ShowLength controls whether to show original length
	ShowLength bool
	// MinMaskLength is minimum number of mask characters to show
	MinMaskLength int
	// MaxShowChars is maximum number of original characters to show
	MaxShowChars int
}

// NewMasker creates a new masker with default settings
func NewMasker() *Masker {
	return &Masker{
		MaskChar:      '*',
		ShowLength:    true,
		MinMaskLength: 8,
		MaxShowChars:  4,
	}
}

// MaskValue masks a sensitive value based on its length and content
func (m *Masker) MaskValue(value string) string {
	if value == "" {
		return value
	}

	// For very short values, mask completely
	if len(value) <= 3 {
		return strings.Repeat(string(m.MaskChar), m.MinMaskLength)
	}

	// For longer values, show first few characters
	showChars := m.MaxShowChars
	if len(value) < showChars*2 {
		showChars = 1
	}

	prefix := value[:showChars]
	maskLength := m.MinMaskLength
	if m.ShowLength && len(value) > m.MinMaskLength {
		maskLength = len(value) - showChars
	}

	return prefix + strings.Repeat(string(m.MaskChar), maskLength)
}

// MaskLogKeyVals masks sensitive information in key-value pairs used for logging
func (m *Masker) MaskLogKeyVals(keyvals []interface{}) []interface{} {
	if len(keyvals) == 0 {
		return keyvals
	}

	result := make([]interface{}, len(keyvals))
	copy(result, keyvals)

	// Process key-value pairs
	for i := 0; i < len(result)-1; i += 2 {
		if key, ok := result[i].(string); ok {
			if m.isSensitiveKey(key) {
				if value, ok := result[i+1].(string); ok {
					result[i+1] = m.MaskValue(value)
				} else {
					// For non-string values, replace with masked placeholder
					result[i+1] = strings.Repeat(string(m.MaskChar), m.MinMaskLength)
				}
			} else if value, ok := result[i+1].(string); ok {
				// Check if the value itself contains sensitive patterns
				if m.containsSensitivePattern(value) {
					result[i+1] = m.MaskValue(value)
				}
			}
		}
	}

	return result
}

// MaskHeaders masks sensitive information in HTTP headers
func (m *Masker) MaskHeaders(headers map[string][]string) map[string][]string {
	if headers == nil {
		return headers
	}

	result := make(map[string][]string)
	for key, values := range headers {
		if m.isSensitiveHeader(key) {
			maskedValues := make([]string, len(values))
			for i, value := range values {
				maskedValues[i] = m.MaskValue(value)
			}
			result[key] = maskedValues
		} else {
			// Check if any value contains sensitive patterns
			maskedValues := make([]string, len(values))
			for i, value := range values {
				if m.containsSensitivePattern(value) {
					maskedValues[i] = m.MaskValue(value)
				} else {
					maskedValues[i] = value
				}
			}
			result[key] = maskedValues
		}
	}

	return result
}

// MaskURL masks sensitive information in URLs (passwords in connection strings)
func (m *Masker) MaskURL(url string) string {
	// Look for password patterns in URLs like postgres://user:password@host/db
	for _, pattern := range SensitiveFields.ValuePatterns {
		if pattern.MatchString(url) {
			url = pattern.ReplaceAllStringFunc(url, func(match string) string {
				return m.MaskValue(match)
			})
		}
	}
	return url
}

// isSensitiveKey checks if a key name indicates sensitive data
func (m *Masker) isSensitiveKey(key string) bool {
	lowerKey := strings.ToLower(key)
	for _, sensitiveField := range SensitiveFields.FieldNames {
		if strings.Contains(lowerKey, sensitiveField) {
			return true
		}
	}
	return false
}

// isSensitiveHeader checks if a header name indicates sensitive data
func (m *Masker) isSensitiveHeader(header string) bool {
	lowerHeader := strings.ToLower(header)
	for _, sensitiveHeader := range SensitiveFields.HeaderNames {
		if strings.Contains(lowerHeader, sensitiveHeader) {
			return true
		}
	}
	return false
}

// containsSensitivePattern checks if a value matches sensitive patterns
func (m *Masker) containsSensitivePattern(value string) bool {
	for _, pattern := range SensitiveFields.ValuePatterns {
		if pattern.MatchString(value) {
			return true
		}
	}
	return false
}

// DefaultMasker provides a global default masker instance
var DefaultMasker = NewMasker()

// MaskLogKeyVals is a convenience function using the default masker
func MaskLogKeyVals(keyvals []interface{}) []interface{} {
	return DefaultMasker.MaskLogKeyVals(keyvals)
}

// MaskValue is a convenience function using the default masker
func MaskValue(value string) string {
	return DefaultMasker.MaskValue(value)
}

// MaskHeaders is a convenience function using the default masker
func MaskHeaders(headers map[string][]string) map[string][]string {
	return DefaultMasker.MaskHeaders(headers)
}

// MaskURL is a convenience function using the default masker
func MaskURL(url string) string {
	return DefaultMasker.MaskURL(url)
}
