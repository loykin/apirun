package httpc

import (
	"context"
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// helper to perform a simple GET using our client
func doGet(t *testing.T, ctx context.Context, url string) (int, error) {
	t.Helper()
	c := New(ctx)
	resp, err := c.R().SetContext(ctx).Get(url)
	if err != nil {
		return 0, err
	}
	return resp.StatusCode(), nil
}

// helper to create a context with TLS min/max bounds
func withTLSBounds(ctx context.Context, min, max string) context.Context {
	if strings.TrimSpace(min) != "" {
		ctx = context.WithValue(ctx, CtxTLSMinVersionKey, min)
	}
	if strings.TrimSpace(max) != "" {
		ctx = context.WithValue(ctx, CtxTLSMaxVersionKey, max)
	}
	return ctx
}

func TestHTTPClient_Insecure_AllowsSelfSigned(t *testing.T) {
	// Self-signed TLS server
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		_, _ = w.Write([]byte("ok"))
	}))
	defer srv.Close()

	// default (no mode) should fail due to unknown authority
	if _, err := doGet(t, context.Background(), srv.URL); err == nil {
		t.Fatalf("expected error without insecure TLS, got nil")
	}

	// insecure should succeed
	ctx := context.WithValue(context.Background(), CtxTLSInsecureKey, true)
	if code, err := doGet(t, ctx, srv.URL); err != nil || code != 200 {
		t.Fatalf("expected 200 with insecure, got code=%d err=%v", code, err)
	}
}

func TestHTTPClient_TLSConfigAppliedToClient(t *testing.T) {
	// insecure: expect TLS config set and InsecureSkipVerify true
	cInsec := New(context.WithValue(context.Background(), CtxTLSInsecureKey, true))
	tr, _ := cInsec.GetClient().Transport.(*http.Transport)
	if tr == nil || tr.TLSClientConfig == nil || tr.TLSClientConfig.InsecureSkipVerify != true {
		t.Fatalf("expected InsecureSkipVerify=true for insecure mode")
	}
	// tls1.2: expect Min=Max=TLS1.2
	c12 := New(withTLSBounds(context.Background(), "1.2", "1.2"))
	tr, _ = c12.GetClient().Transport.(*http.Transport)
	if tr == nil || tr.TLSClientConfig == nil {
		t.Fatalf("expected TLSClientConfig for tls1.2 mode")
	}
	if tr.TLSClientConfig.MinVersion != tls.VersionTLS12 || tr.TLSClientConfig.MaxVersion != tls.VersionTLS12 {
		t.Fatalf("expected TLS1.2 only, got Min=%v Max=%v", tr.TLSClientConfig.MinVersion, tr.TLSClientConfig.MaxVersion)
	}
	// tls1.3: expect Min=Max=TLS1.3
	c13 := New(withTLSBounds(context.Background(), "1.3", "1.3"))
	tr, _ = c13.GetClient().Transport.(*http.Transport)
	if tr == nil || tr.TLSClientConfig == nil {
		t.Fatalf("expected TLSClientConfig for tls1.3 mode")
	}
	if tr.TLSClientConfig.MinVersion != tls.VersionTLS13 || tr.TLSClientConfig.MaxVersion != tls.VersionTLS13 {
		t.Fatalf("expected TLS1.3 only, got Min=%v Max=%v", tr.TLSClientConfig.MinVersion, tr.TLSClientConfig.MaxVersion)
	}
	// auto/default: we do not set TLS config (leave resty default)
	cAuto := New(context.Background())
	trAuto, _ := cAuto.GetClient().Transport.(*http.Transport)
	if trAuto != nil && trAuto.TLSClientConfig != nil {
		// Resty may reuse a default transport with nil TLS config; we only assert that we didn't force values.
		if trAuto.TLSClientConfig.MinVersion != 0 || trAuto.TLSClientConfig.MaxVersion != 0 || trAuto.TLSClientConfig.InsecureSkipVerify {
			t.Fatalf("expected default TLS config not to be constrained")
		}
	}
}

func TestHTTPClient_Auto_DefaultMode(t *testing.T) {
	// Plain HTTP server should always work; ensure we don't set TLS and break normal http
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(204)
	}))
	defer srv.Close()
	if !strings.HasPrefix(srv.URL, "http://") {
		t.Fatalf("expected http server URL, got %s", srv.URL)
	}
	if code, err := doGet(t, context.Background(), srv.URL); err != nil || code != 204 {
		t.Fatalf("default client to http server expected 204, got code=%d err=%v", code, err)
	}
}
