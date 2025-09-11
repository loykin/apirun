package httpc

import (
	"crypto/tls"

	"github.com/go-resty/resty/v2"
)

type Httpc struct {
	TlsConfig *tls.Config
}

// New returns a resty.Client configured according to the receiver's TLS settings.
// Note: when MinVersion/MaxVersion are zero, Go's http defaults are used (no forced TLS1.3).
func (h *Httpc) New() *resty.Client {
	c := resty.New()
	cfg := h.TlsConfig
	if cfg == nil {
		return c
	}
	// Apply TLS config via resty and ensure underlying client transport is set
	c.SetTLSClientConfig(cfg)
	return c
}
