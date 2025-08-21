package auth

import (
	"strings"
	"sync"
)

type Token struct {
	Header string
	Value  string
}

// tokenStore encapsulates the map and its mutex into a single structure.
type tokenStore struct {
	mu     sync.RWMutex
	byName map[string]Token
}

func newTokenStore() *tokenStore {
	return &tokenStore{byName: make(map[string]Token)}
}

func (s *tokenStore) set(name, header, value string) {
	name = strings.ToLower(strings.TrimSpace(name))
	if name == "" || value == "" || header == "" {
		return
	}
	s.mu.Lock()
	s.byName[name] = Token{Header: header, Value: value}
	s.mu.Unlock()
}

func (s *tokenStore) get(name string) (header, value string, ok bool) {
	name = strings.ToLower(strings.TrimSpace(name))
	s.mu.RLock()
	tok, exists := s.byName[name]
	s.mu.RUnlock()
	if !exists {
		return "", "", false
	}
	return tok.Header, tok.Value, true
}

func (s *tokenStore) clear() {
	s.mu.Lock()
	s.byName = make(map[string]Token)
	s.mu.Unlock()
}

var globalTokens = newTokenStore()

// SetToken stores a token under a logical name (e.g., "keycloak").
// Header is the header name to use (e.g., Authorization or X-Api-Key),
// Value is the header value (may include Bearer prefix if needed).
func SetToken(name, header, value string) {
	globalTokens.set(name, header, value)
}

// GetToken retrieves a stored token by name.
func GetToken(name string) (header, value string, ok bool) {
	return globalTokens.get(name)
}

// ClearTokens removes all stored tokens (useful for tests).
func ClearTokens() {
	globalTokens.clear()
}

func headerOrDefault(h string) string {
	h = strings.TrimSpace(h)
	if h == "" {
		return "Authorization"
	}
	return h
}

// extractJSONField is a tiny helper to get a top-level string field without adding a new dependency.
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
