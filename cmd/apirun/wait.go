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

// doWait polls an HTTP endpoint until it returns the expected status or timeout elapses.
//
// Behavior:
// - method defaults to GET; supports GET and HEAD (others fallback to GET)
// - expected status defaults to 200
// - timeout defaults to 60s; interval defaults to 2s
// - url is rendered with Go template using provided env
// - TLS client options are applied via clientCfg and attached to the polling context
func doWait(ctx context.Context, env *env.Env, wc WaitConfig, clientCfg ClientConfig) error {
	urlRaw := strings.TrimSpace(wc.URL)
	if urlRaw == "" {
		return nil
	}
	method := strings.ToUpper(strings.TrimSpace(wc.Method))
	if method == "" {
		method = "GET"
	}
	expected := wc.Status
	if expected == 0 {
		expected = 200
	}
	timeout := 60 * time.Second
	if s := strings.TrimSpace(wc.Timeout); s != "" {
		if d, err := time.ParseDuration(s); err == nil {
			timeout = d
		}
	}
	interval := 2 * time.Second
	if s := strings.TrimSpace(wc.Interval); s != "" {
		if d, err := time.ParseDuration(s); err == nil {
			interval = d
		}
	}

	urlToHit := env.RenderGoTemplate(urlRaw)
	ctxWait := ctx
	// Prepare TLS options for the wait HTTP client via httpc.Httpc
	minV := uint16(0)
	maxV := uint16(0)
	switch strings.TrimSpace(strings.ToLower(clientCfg.MinTLSVersion)) {
	case "1.0", "10", "tls1.0", "tls10":
		minV = tls.VersionTLS10
	case "1.1", "11", "tls1.1", "tls11":
		minV = tls.VersionTLS11
	case "1.2", "12", "tls1.2", "tls12":
		minV = tls.VersionTLS12
	case "1.3", "13", "tls1.3", "tls13":
		minV = tls.VersionTLS13
	}
	switch strings.TrimSpace(strings.ToLower(clientCfg.MaxTLSVersion)) {
	case "1.0", "10", "tls1.0", "tls10":
		maxV = tls.VersionTLS10
	case "1.1", "11", "tls1.1", "tls11":
		maxV = tls.VersionTLS11
	case "1.2", "12", "tls1.2", "tls12":
		maxV = tls.VersionTLS12
	case "1.3", "13", "tls1.3", "tls13":
		maxV = tls.VersionTLS13
	}

	// for legacy compatibility, if no max version is set, use min version
	// #nosec G402 -- legacy compatibility only, do not use in production
	cfg := &tls.Config{MinVersion: minV, MaxVersion: maxV}
	if clientCfg.Insecure {
		// #nosec G402 — Intentionally allow self-signed certificates for the wait probe when explicitly configured
		cfg.InsecureSkipVerify = true
	}
	hcfg := httpc.Httpc{TlsConfig: cfg}

	deadline := time.Now().Add(timeout)
	var lastStatus int
	for {
		client := hcfg.New()
		req := client.R().SetContext(ctxWait)
		var status int
		var err error
		switch method {
		case "GET":
			resp, e := req.Get(urlToHit)
			err = e
			if resp != nil {
				status = resp.StatusCode()
			}
		case "HEAD":
			resp, e := req.Head(urlToHit)
			err = e
			if resp != nil {
				status = resp.StatusCode()
			}
		default:
			resp, e := req.Get(urlToHit)
			err = e
			if resp != nil {
				status = resp.StatusCode()
			}
		}
		if err == nil && status == expected {
			return nil
		}
		lastStatus = status
		if time.Now().After(deadline) {
			return fmt.Errorf("wait: timeout waiting for %s to return %d (last=%d)", urlToHit, expected, lastStatus)
		}
		time.Sleep(interval)
	}
}
