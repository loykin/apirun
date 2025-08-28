package main

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/loykin/apimigrate"
)

// doWait polls an HTTP endpoint until it returns the expected status or timeout elapses.
//
// Behavior:
// - method defaults to GET; supports GET and HEAD (others fallback to GET)
// - expected status defaults to 200
// - timeout defaults to 60s; interval defaults to 2s
// - url is rendered with Go template using provided env
// - TLS client options are applied via clientCfg and attached to the polling context
func doWait(ctx context.Context, env apimigrate.Env, wc WaitConfig, clientCfg ClientConfig, verbose bool) error {
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
	// Apply TLS options for the wait HTTP client
	if clientCfg.Insecure {
		ctxWait = apimigrate.WithTLSInsecure(ctxWait, true)
	}
	if s := strings.TrimSpace(clientCfg.MinTLSVersion); s != "" {
		ctxWait = apimigrate.WithTLSMinVersion(ctxWait, s)
	}
	if s := strings.TrimSpace(clientCfg.MaxTLSVersion); s != "" {
		ctxWait = apimigrate.WithTLSMaxVersion(ctxWait, s)
	}

	if verbose {
		log.Printf("waiting for %s %s to return %d (timeout=%s, interval=%s)", method, urlToHit, expected, timeout, interval)
	}
	deadline := time.Now().Add(timeout)
	var lastStatus int
	for {
		client := apimigrate.NewHTTPClient(ctxWait)
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
			if verbose {
				log.Printf("wait condition met: %d", status)
			}
			return nil
		}
		lastStatus = status
		if time.Now().After(deadline) {
			return fmt.Errorf("wait: timeout waiting for %s to return %d (last=%d)", urlToHit, expected, lastStatus)
		}
		time.Sleep(interval)
	}
}
