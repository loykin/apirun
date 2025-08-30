package auth

import (
	"strings"
	"sync"
)

type Token struct {
	Value string
}

// TokenStore encapsulates the map and its mutex into a single structure.
type TokenStore struct {
	mu     sync.RWMutex
	byName map[string]Token
}

func (s *TokenStore) set(name, value string) {
	name = strings.ToLower(strings.TrimSpace(name))
	if name == "" || value == "" {
		return
	}
	s.mu.Lock()
	s.byName[name] = Token{Value: value}
	s.mu.Unlock()
}

func (s *TokenStore) get(name string) (value string, ok bool) {
	name = strings.ToLower(strings.TrimSpace(name))
	s.mu.RLock()
	tok, exists := s.byName[name]
	s.mu.RUnlock()
	if !exists {
		return "", false
	}
	return tok.Value, true
}

func (s *TokenStore) clear() {
	s.mu.Lock()
	s.byName = make(map[string]Token)
	s.mu.Unlock()
}
