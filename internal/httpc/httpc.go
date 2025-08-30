package httpc

import (
	"crypto/tls"

	"github.com/go-resty/resty/v2"
)

type Httpc struct {
	TlsConfig *tls.Config
}

// New returns a resty.Client configured according to the receiver's TLS settings.
// Defaults: MinVersion TLS1.3 when MinVersion is zero.
func (h *Httpc) New() *resty.Client {
	c := resty.New()
	cfg := h.TlsConfig
	if cfg == nil {
		return c
	}
	if cfg.MinVersion == 0 {
		cfg.MinVersion = tls.VersionTLS13
	}
	// Apply TLS config via resty and ensure underlying client transport is set
	c.SetTLSClientConfig(cfg)
	return c
}
