package util

import (
	"reflect"
	"strings"
)

// TrimSpaceFields trims whitespace from multiple string fields
func TrimSpaceFields(fields ...string) []string {
	result := make([]string, len(fields))
	for i, field := range fields {
		result[i] = strings.TrimSpace(field)
	}
	return result
}

// TrimAndLower trims whitespace and converts to lowercase
func TrimAndLower(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}

// TrimEmptyCheck trims whitespace and checks if non-empty
func TrimEmptyCheck(s string) (string, bool) {
	trimmed := strings.TrimSpace(s)
	return trimmed, trimmed != ""
}

// TrimWithDefault trims whitespace and returns default if empty
func TrimWithDefault(s, defaultValue string) string {
	trimmed := strings.TrimSpace(s)
	if trimmed == "" {
		return defaultValue
	}
	return trimmed
}

// TrimStructFields automatically trims all string fields in a struct using reflection
func TrimStructFields(v interface{}) {
	rv := reflect.ValueOf(v)
	if rv.Kind() == reflect.Ptr {
		rv = rv.Elem()
	}

	if rv.Kind() != reflect.Struct {
		return
	}

	rt := rv.Type()
	for i := 0; i < rv.NumField(); i++ {
		field := rv.Field(i)
		fieldType := rt.Field(i)

		// Skip unexported fields
		if !fieldType.IsExported() {
			continue
		}

		if field.Kind() == reflect.String && field.CanSet() {
			field.SetString(strings.TrimSpace(field.String()))
		}
	}
}

// ConfigFields holds commonly trimmed configuration fields
type ConfigFields struct {
	Type     string
	Name     string
	Host     string
	User     string
	Password string
	DBName   string
	SSLMode  string
	Path     string
}

// Trim trims all fields in ConfigFields
func (c *ConfigFields) Trim() {
	TrimStructFields(c)
}
