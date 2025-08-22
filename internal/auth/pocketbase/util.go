package pocketbase

import "strings"

// ExtractJSONField is a tiny helper to get a top-level string field without adding a new dependency.
// It is not a general JSON parser; it looks for a pattern like "\"field\":\"value\"" or with spacing.
// For robustness, in the rest of the project we already use tidwall/gjson; but to keep this
// package dependency-light, we avoid adding imports and keep a best-effort extraction here.
func extractJSONField(b []byte, field string) string {
	s := string(b)
	// Normalize field name
	needle := `"` + field + `"`
	idx := strings.Index(s, needle)
	if idx == -1 {
		return ""
	}
	s = s[idx+len(needle):]
	colon := strings.Index(s, ":")
	if colon == -1 {
		return ""
	}
	s = s[colon+1:]
	// Trim and handle possible quotes
	s = strings.TrimSpace(s)
	if len(s) == 0 {
		return ""
	}
	if s[0] == '"' {
		// string value
		end := strings.Index(s[1:], "\"")
		if end == -1 {
			return ""
		}
		return s[1 : 1+end]
	}
	// Non-quoted; read until comma or brace
	end := len(s)
	if comma := strings.IndexAny(s, ",}"); comma != -1 {
		end = comma
	}
	return strings.TrimSpace(s[:end])
}
