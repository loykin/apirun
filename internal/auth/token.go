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
