package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"strings"
	"time"

	"github.com/loykin/apirun/internal/httpc"
	"github.com/loykin/apirun/pkg/env"
)

const (
	DefaultWaitTimeout  = 60 * time.Second
	DefaultWaitInterval = 2 * time.Second
)

// parseTLSVersion converts a TLS version string to the corresponding crypto/tls constant.
// Supports various formats: "1.0", "10", "tls1.0", "tls10", etc.
// Returns 0 if the version string is not recognized.
func parseTLSVersion(version string) uint16 {
	switch strings.TrimSpace(strings.ToLower(version)) {
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
func parseWaitConfig(wc WaitConfig, env *env.Env) waitParams {
	urlRaw := strings.TrimSpace(wc.URL)

	method := strings.ToUpper(strings.TrimSpace(wc.Method))
	if method == "" {
		method = "GET"
	}

	expected := wc.Status
	if expected == 0 {
		expected = 200
	}

	timeout := DefaultWaitTimeout
	if s := strings.TrimSpace(wc.Timeout); s != "" {
		if d, err := time.ParseDuration(s); err == nil {
			timeout = d
		}
	}

	interval := DefaultWaitInterval
	if s := strings.TrimSpace(wc.Interval); s != "" {
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
func setupTLSConfig(clientCfg ClientConfig) *tls.Config {
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

		time.Sleep(params.interval)
	}
}

// doWait polls an HTTP endpoint until it returns the expected status or timeout elapses.
//
// Behavior:
// - method defaults to GET; supports GET and HEAD (others fallback to GET)
// - expected status defaults to 200
// - timeout defaults to 60s; interval defaults to 2s
// - url is rendered with Go template using provided env
// - TLS client options are applied via clientCfg and attached to the polling context
func doWait(ctx context.Context, env *env.Env, wc WaitConfig, clientCfg ClientConfig) error {
	// Early exit if no URL is provided
	if strings.TrimSpace(wc.URL) == "" {
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
