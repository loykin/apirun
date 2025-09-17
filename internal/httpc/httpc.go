package httpc

import (
	"crypto/tls"

	"github.com/go-resty/resty/v2"
	"github.com/loykin/apimigrate/internal/common"
)

type Httpc struct {
	TlsConfig *tls.Config
}

// New returns a resty.Client configured according to the receiver's TLS settings.
// Note: when MinVersion/MaxVersion are zero, Go's http defaults are used (no forced TLS1.3).
func (h *Httpc) New() *resty.Client {
	logger := common.GetLogger().WithComponent("httpc")
	logger.Debug("creating new HTTP client")

	c := resty.New()

	// Add request/response logging middleware
	c.OnBeforeRequest(func(c *resty.Client, req *resty.Request) error {
		logger.Debug("HTTP request",
			"method", req.Method,
			"url", req.URL)
		return nil
	})

	c.OnAfterResponse(func(c *resty.Client, resp *resty.Response) error {
		logger.Info("HTTP response",
			"method", resp.Request.Method,
			"url", resp.Request.URL,
			"status_code", resp.StatusCode(),
			"duration_ms", resp.Time().Milliseconds())
		return nil
	})

	cfg := h.TlsConfig
	if cfg == nil {
		logger.Debug("using default HTTP client configuration (no TLS config)")
		return c
	}
	// Apply TLS config via resty and ensure underlying client transport is set
	logger.Debug("applying TLS configuration to HTTP client",
		"insecure_skip_verify", cfg.InsecureSkipVerify,
		"min_version", cfg.MinVersion,
		"max_version", cfg.MaxVersion)
	c.SetTLSClientConfig(cfg)
	logger.Debug("HTTP client created with TLS configuration")
	return c
}
