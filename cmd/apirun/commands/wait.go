package commands

import (
	"context"
	"crypto/tls"
	"fmt"
	"strings"
	"time"

	"github.com/loykin/apirun/cmd/apirun/config"
	"github.com/loykin/apirun/internal/constants"
	"github.com/loykin/apirun/internal/httpc"
	"github.com/loykin/apirun/internal/util"
	"github.com/loykin/apirun/pkg/env"
)

// parseTLSVersion converts a TLS version string to the corresponding crypto/tls constant.
// Supports various formats: "1.0", "10", "tls1.0", "tls10", etc.
// Returns 0 if the version string is not recognized.
func parseTLSVersion(version string) uint16 {
	switch util.TrimAndLower(version) {
	case "1.0", "10", "tls1.0", "tls10":
		return tls.VersionTLS10
	case "1.1", "11", "tls1.1", "tls11":
		return tls.VersionTLS11
	case "1.2", "12", "tls1.2", "tls12":
		return tls.VersionTLS12
	case "1.3", "13", "tls1.3", "tls13":
		return tls.VersionTLS13
	default:
		return 0
	}
}

// waitParams holds the parsed and normalized parameters for waiting
type waitParams struct {
	url      string
	method   string
	expected int
	timeout  time.Duration
	interval time.Duration
}

// parseWaitConfig parses and normalizes wait configuration with defaults
func parseWaitConfig(wc config.WaitConfig, env *env.Env) waitParams {
	urlRaw, _ := util.TrimEmptyCheck(wc.URL)

	method := strings.ToUpper(util.TrimWithDefault(wc.Method, constants.DefaultWaitMethod))

	expected := wc.Status
	if expected == 0 {
		expected = constants.DefaultWaitStatus
	}

	timeout := constants.DefaultWaitTimeout
	if s, hasTimeout := util.TrimEmptyCheck(wc.Timeout); hasTimeout {
		if d, err := time.ParseDuration(s); err == nil {
			timeout = d
		}
	}

	interval := constants.DefaultWaitInterval
	if s, hasInterval := util.TrimEmptyCheck(wc.Interval); hasInterval {
		if d, err := time.ParseDuration(s); err == nil {
			interval = d
		}
	}

	url := env.RenderGoTemplate(urlRaw)

	return waitParams{
		url:      url,
		method:   method,
		expected: expected,
		timeout:  timeout,
		interval: interval,
	}
}

// setupTLSConfig creates TLS configuration from client config
func setupTLSConfig(clientCfg config.ClientConfig) *tls.Config {
	minV := parseTLSVersion(clientCfg.MinTLSVersion)
	maxV := parseTLSVersion(clientCfg.MaxTLSVersion)

	// for legacy compatibility, if no max version is set, use min version
	// #nosec G402 -- legacy compatibility only, do not use in production
	cfg := &tls.Config{MinVersion: minV, MaxVersion: maxV}
	if clientCfg.Insecure {
		// #nosec G402 — Intentionally allow self-signed certificates for the wait probe when explicitly configured
		cfg.InsecureSkipVerify = true
	}
	return cfg
}

// performHTTPRequest executes an HTTP request with the specified method
func performHTTPRequest(ctx context.Context, hcfg *httpc.Httpc, method, url string) (int, error) {
	client := hcfg.New()
	req := client.R().SetContext(ctx)

	var status int
	var err error

	switch method {
	case "GET":
		resp, e := req.Get(url)
		err = e
		if resp != nil {
			status = resp.StatusCode()
		}
	case "HEAD":
		resp, e := req.Head(url)
		err = e
		if resp != nil {
			status = resp.StatusCode()
		}
	default:
		resp, e := req.Get(url)
		err = e
		if resp != nil {
			status = resp.StatusCode()
		}
	}

	return status, err
}

// performPolling repeatedly polls the endpoint until success or timeout
func performPolling(ctx context.Context, hcfg *httpc.Httpc, params waitParams) error {
	deadline := time.Now().Add(params.timeout)
	var lastStatus int

	for {
		status, err := performHTTPRequest(ctx, hcfg, params.method, params.url)

		if err == nil && status == params.expected {
			return nil
		}

		lastStatus = status
		if time.Now().After(deadline) {
			return fmt.Errorf("wait: timeout waiting for %s to return %d (last=%d)",
				params.url, params.expected, lastStatus)
		}

		// Context-aware sleep
		timer := time.NewTimer(params.interval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
			// Continue polling
		}
	}
}

// DoWait polls an HTTP endpoint until it returns the expected status or timeout elapses.
//
// Behavior:
// - method defaults to GET; supports GET and HEAD (others fallback to GET)
// - expected status defaults to 200
// - timeout defaults to 60s; interval defaults to 2s
// - url is rendered with Go template using provided env
// - TLS client options are applied via clientCfg and attached to the polling context
func DoWait(ctx context.Context, env *env.Env, wc config.WaitConfig, clientCfg config.ClientConfig) error {
	// Early exit if no URL is provided
	if _, hasURL := util.TrimEmptyCheck(wc.URL); !hasURL {
		return nil
	}

	// Parse and normalize wait configuration
	params := parseWaitConfig(wc, env)

	// Setup TLS configuration
	tlsConfig := setupTLSConfig(clientCfg)
	hcfg := &httpc.Httpc{TlsConfig: tlsConfig}

	// Perform polling until success or timeout
	return performPolling(ctx, hcfg, params)
}
