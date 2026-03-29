package common

import (
	"crypto/tls"
	"sync/atomic"
)

// Package-level TLS configuration used by auth HTTP clients (oauth2, pocketbase, etc.).
// This mirrors the task-level TLS configuration so that auth flows honor the same settings.
var tlsConfig atomic.Pointer[tls.Config]

// SetTLSConfig sets the TLS configuration for auth HTTP clients.
func SetTLSConfig(cfg *tls.Config) { tlsConfig.Store(cfg) }

// GetTLSConfig returns the currently configured TLS settings for auth HTTP clients.
func GetTLSConfig() *tls.Config { return tlsConfig.Load() }
