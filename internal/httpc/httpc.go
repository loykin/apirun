package httpc

import (
	"context"
	"crypto/tls"
	"strings"

	"github.com/go-resty/resty/v2"
)

// Context key names used by the CLI when loading config
const (
	// Explicit TLS config keys
	CtxTLSInsecureKey   = "apimigrate.tls_insecure"
	CtxTLSMinVersionKey = "apimigrate.tls_min_version"
	CtxTLSMaxVersionKey = "apimigrate.tls_max_version"
)

func parseTLSVersion(s string) uint16 {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.TrimPrefix(s, "tls") // e.g., tls1.2 -> 1.2
	if s == "" {
		return 0
	}
	switch s {
	case "1.0", "10":
		return tls.VersionTLS10
	case "1.1", "11":
		return tls.VersionTLS11
	case "1.2", "12":
		return tls.VersionTLS12
	case "1.3", "13":
		return tls.VersionTLS13
	default:
		return 0
	}
}

// New returns a resty.Client configured according to TLS settings in context.
// Preference order:
// 1) New explicit keys: apimigrate.tls_insecure, tls_min_version, tls_max_version
// 2) Legacy CtxTLSModeKey string: insecure|tls1.2|tls1.3|auto
func New(ctx context.Context) *resty.Client {
	c := resty.New()

	// Check new explicit keys first
	var wantCfg bool
	cfg := &tls.Config{}
	if v := ctx.Value(CtxTLSInsecureKey); v != nil {
		if b, ok := v.(bool); ok {
			cfg.InsecureSkipVerify = b
			// Presence of explicit key means we want to apply TLS config, regardless of value
			wantCfg = true
		}
	}
	if v := ctx.Value(CtxTLSMinVersionKey); v != nil {
		if s, ok := v.(string); ok {
			if ver := parseTLSVersion(s); ver != 0 {
				cfg.MinVersion = ver
				wantCfg = true
			}
		}
	}
	if v := ctx.Value(CtxTLSMaxVersionKey); v != nil {
		if s, ok := v.(string); ok {
			if ver := parseTLSVersion(s); ver != 0 {
				cfg.MaxVersion = ver
				wantCfg = true
			}
		}
	}
	if wantCfg {
		c.SetTLSClientConfig(cfg)
		return c
	}

	// No legacy fallback; if no explicit keys provided, use default Go TLS settings
	return c
}
