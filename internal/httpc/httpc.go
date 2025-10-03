package httpc

import (
	"crypto/tls"
	"net"
	"net/http"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/loykin/apirun/internal/common"
	"github.com/loykin/apirun/internal/constants"
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

	// Configure optimized HTTP transport with connection pooling
	transport := &http.Transport{
		DialContext: (&net.Dialer{
			Timeout:   constants.DefaultHTTPDialTimeout,
			KeepAlive: constants.DefaultHTTPKeepAliveTimeout,
		}).DialContext,
		TLSHandshakeTimeout:   constants.DefaultHTTPTLSHandshakeTimeout,
		MaxIdleConns:          constants.DefaultHTTPMaxIdleConns,
		MaxIdleConnsPerHost:   constants.DefaultHTTPMaxIdleConnsPerHost,
		MaxConnsPerHost:       constants.DefaultHTTPMaxConnsPerHost,
		IdleConnTimeout:       constants.DefaultHTTPIdleConnTimeout,
		DisableCompression:    false, // Enable compression for efficiency
		ExpectContinueTimeout: 1 * time.Second,
	}

	// Set the optimized transport and request timeout
	c.SetTransport(transport).
		SetTimeout(constants.DefaultHTTPRequestTimeout)

	logger.Debug("HTTP client configured with optimized connection pool",
		"max_idle_conns", constants.DefaultHTTPMaxIdleConns,
		"max_idle_conns_per_host", constants.DefaultHTTPMaxIdleConnsPerHost,
		"max_conns_per_host", constants.DefaultHTTPMaxConnsPerHost,
		"dial_timeout", constants.DefaultHTTPDialTimeout,
		"request_timeout", constants.DefaultHTTPRequestTimeout)

	// Configure retry policy for resilient HTTP operations
	c.SetRetryCount(3).
		SetRetryWaitTime(1 * time.Second).
		SetRetryMaxWaitTime(5 * time.Second).
		AddRetryCondition(func(r *resty.Response, err error) bool {
			// Retry on network errors
			if err != nil {
				logger.Debug("retrying due to network error", "error", err)
				return true
			}
			// Retry on 5xx server errors (like 502 Bad Gateway)
			if r.StatusCode() >= 500 {
				logger.Debug("retrying due to server error", "status_code", r.StatusCode())
				return true
			}
			// Retry on specific 4xx errors that might be transient
			if r.StatusCode() == http.StatusTooManyRequests || r.StatusCode() == http.StatusRequestTimeout {
				logger.Debug("retrying due to transient client error", "status_code", r.StatusCode())
				return true
			}
			return false
		})

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
			"duration_ms", resp.Time().Milliseconds(),
			"attempt", resp.Request.Attempt)
		return nil
	})

	// Log retry information through the AfterResponse middleware
	logger.Debug("HTTP client configured with retry policy",
		"max_retries", 3,
		"initial_wait", "1s",
		"max_wait", "5s")

	cfg := h.TlsConfig
	if cfg != nil {
		// Apply TLS config to our optimized transport
		transport.TLSClientConfig = cfg
		logger.Debug("applying TLS configuration to HTTP client",
			"insecure_skip_verify", cfg.InsecureSkipVerify,
			"min_version", cfg.MinVersion,
			"max_version", cfg.MaxVersion)
	} else {
		logger.Debug("using default TLS configuration")
	}

	logger.Debug("HTTP client created with optimized transport and connection pooling")
	return c
}
