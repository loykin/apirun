package common

import "crypto/tls"

// Package-level TLS configuration used by auth HTTP clients (oauth2, pocketbase, etc.).
// This mirrors the task-level TLS configuration so that auth flows honor the same settings.
var tlsConfig *tls.Config

// SetTLSConfig sets the TLS configuration for auth HTTP clients.
func SetTLSConfig(cfg *tls.Config) { tlsConfig = cfg }

// GetTLSConfig returns the currently configured TLS settings for auth HTTP clients.
func GetTLSConfig() *tls.Config { return tlsConfig }
