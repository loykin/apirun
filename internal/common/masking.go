package common

import (
	"fmt"
	"regexp"
	"strings"
)

// SensitivePattern represents a pattern to detect and mask sensitive information
type SensitivePattern struct {
	Name        string         // Pattern name (e.g., "password", "api_key")
	Regex       *regexp.Regexp // Regular expression to match sensitive data
	Replacement string         // Replacement string (e.g., "***MASKED***")
	Keys        []string       // Specific keys to mask (case-insensitive)
}

// DefaultSensitivePatterns contains common patterns for sensitive information
var DefaultSensitivePatterns = []SensitivePattern{
	{
		Name:        "password",
		Regex:       regexp.MustCompile(`(?i)(password|passwd|pwd)["'\s]*[:=]["'\s]*([^"',}\]\s]+)`),
		Replacement: `${1}":"***MASKED***"`,
		Keys:        []string{"password", "passwd", "pwd"},
	},
	{
		Name:        "api_key",
		Regex:       regexp.MustCompile(`(?i)(api[_-]?key|apikey)["'\s]*[:=]["'\s]*([^"',}\]\s]+)`),
		Replacement: `${1}":"***MASKED***"`,
		Keys:        []string{"api_key", "apikey", "api-key"},
	},
	{
		Name:        "token",
		Regex:       regexp.MustCompile(`(?i)(token|access[_-]?token|auth[_-]?token)["'\s]*[:=]["'\s]*([^"',}\]\s]+)`),
		Replacement: `${1}":"***MASKED***"`,
		Keys:        []string{"token", "access_token", "auth_token", "access-token", "auth-token"},
	},
	{
		Name:        "authorization",
		Regex:       regexp.MustCompile(`(?i)(authorization)["'\s]*[:=]["'\s]*([^"',}\]\s]+)`),
		Replacement: `${1}":"***MASKED***"`,
		Keys:        []string{"authorization"},
	},
	{
		Name:        "bearer_token",
		Regex:       regexp.MustCompile(`(?i)Bearer\s+[A-Za-z0-9\-._~+/]+=*`),
		Replacement: "Bearer ***MASKED***",
		Keys:        []string{},
	},
	{
		Name:        "basic_auth",
		Regex:       regexp.MustCompile(`(?i)Basic\s+[A-Za-z0-9+/]+=*`),
		Replacement: "Basic ***MASKED***",
		Keys:        []string{},
	},
	{
		Name:        "secret",
		Regex:       regexp.MustCompile(`(?i)(secret|client[_-]?secret)["'\s]*[:=]["'\s]*([^"',}\]\s]+)`),
		Replacement: `${1}":"***MASKED***"`,
		Keys:        []string{"secret", "client_secret", "client-secret"},
	},
}

// Masker handles masking of sensitive information in logs
type Masker struct {
	patterns []SensitivePattern
	enabled  bool
}

// NewMasker creates a new masker with default patterns
func NewMasker() *Masker {
	return &Masker{
		patterns: DefaultSensitivePatterns,
		enabled:  true,
	}
}

// NewMaskerWithPatterns creates a new masker with custom patterns
func NewMaskerWithPatterns(patterns []SensitivePattern) *Masker {
	return &Masker{
		patterns: patterns,
		enabled:  true,
	}
}

// SetEnabled enables or disables masking
func (m *Masker) SetEnabled(enabled bool) {
	m.enabled = enabled
}

// IsEnabled returns whether masking is enabled
func (m *Masker) IsEnabled() bool {
	return m.enabled
}

// AddPattern adds a new sensitive pattern
func (m *Masker) AddPattern(pattern SensitivePattern) {
	// Compile regex if not already compiled
	if pattern.Regex == nil {
		// Create regex pattern from keys
		if len(pattern.Keys) > 0 {
			keyPattern := strings.Join(pattern.Keys, "|")
			regexPattern := fmt.Sprintf("(?i)\\b(%s)\\s*[:=]\\s*['\"]?([^'\",\\s}\\]]+)['\"]?", keyPattern)
			pattern.Regex = regexp.MustCompile(regexPattern)
			if pattern.Replacement == "" {
				pattern.Replacement = "$1:\"***MASKED***\""
			}
		}
	}
	m.patterns = append(m.patterns, pattern)
}

// MaskString masks sensitive information in a string
func (m *Masker) MaskString(input string) string {
	if !m.enabled {
		return input
	}

	result := input
	for _, pattern := range m.patterns {
		result = pattern.Regex.ReplaceAllString(result, pattern.Replacement)
	}
	return result
}

// MaskValue masks sensitive information based on key-value context
func (m *Masker) MaskValue(key string, value interface{}) interface{} {
	if !m.enabled {
		return value
	}

	// Convert value to string for processing
	strValue, ok := value.(string)
	if !ok {
		// For non-string values, try to convert to string representation
		strValue = strings.TrimSpace(toString(value))
	}

	// Check if key matches any sensitive patterns
	lowerKey := strings.ToLower(key)
	for _, pattern := range m.patterns {
		for _, sensitiveKey := range pattern.Keys {
			if lowerKey == strings.ToLower(sensitiveKey) {
				return "***MASKED***"
			}
		}
	}

	// Apply regex patterns to the value
	return m.MaskString(strValue)
}

// MaskKeyValuePairs masks sensitive information in key-value pairs
func (m *Masker) MaskKeyValuePairs(pairs ...any) []any {
	if !m.enabled {
		return pairs
	}

	result := make([]any, len(pairs))
	for i := 0; i < len(pairs); i += 2 {
		if i+1 < len(pairs) {
			key := pairs[i]
			value := pairs[i+1]

			// Mask the key if it's a string
			if keyStr, ok := key.(string); ok {
				result[i] = keyStr
				result[i+1] = m.MaskValue(keyStr, value)
			} else {
				result[i] = key
				result[i+1] = value
			}
		} else {
			result[i] = pairs[i]
		}
	}
	return result
}

// toString converts various types to string representation
func toString(v interface{}) string {
	if v == nil {
		return ""
	}

	switch val := v.(type) {
	case string:
		return val
	case []byte:
		return string(val)
	case error:
		return val.Error()
	default:
		return ""
	}
}

// Global masker instance
var globalMasker = NewMasker()

// SetGlobalMasker sets the global masker instance
func SetGlobalMasker(masker *Masker) {
	globalMasker = masker
}

// GetGlobalMasker returns the global masker instance
func GetGlobalMasker() *Masker {
	return globalMasker
}

// MaskSensitiveData masks sensitive data using the global masker
func MaskSensitiveData(input string) string {
	return globalMasker.MaskString(input)
}

// EnableMasking enables/disables global masking
func EnableMasking(enabled bool) {
	globalMasker.SetEnabled(enabled)
}

// IsMaskingEnabled returns whether global masking is enabled
func IsMaskingEnabled() bool {
	return globalMasker.IsEnabled()
}
