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
func doGet(t *testing.T, ctx context.Context, url string, h *Httpc) (int, error) {
	t.Helper()
	var cfg Httpc
	if h != nil {
		cfg = *h
	}
	c := cfg.New()
	resp, err := c.R().SetContext(ctx).Get(url)
	if err != nil {
		return 0, err
	}
	return resp.StatusCode(), nil
}

func TestHTTPClient_Insecure_AllowsSelfSigned(t *testing.T) {
	// Self-signed TLS server
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		_, _ = w.Write([]byte("ok"))
	}))
	defer srv.Close()

	// default (no mode) should fail due to unknown authority
	if _, err := doGet(t, context.Background(), srv.URL, nil); err == nil {
		t.Fatalf("expected error without insecure TLS, got nil")
	}

	// insecure should succeed
	h := &Httpc{TlsConfig: &tls.Config{InsecureSkipVerify: true}}
	if code, err := doGet(t, context.Background(), srv.URL, h); err != nil || code != 200 {
		t.Fatalf("expected 200 with insecure, got code=%d err=%v", code, err)
	}
}

func TestHTTPClient_TLSConfigAppliedToClient(t *testing.T) {
	// insecure: expect TLS config set and InsecureSkipVerify true
	hInsec := &Httpc{TlsConfig: &tls.Config{InsecureSkipVerify: true}}
	cInsec := hInsec.New()
	tr, _ := cInsec.GetClient().Transport.(*http.Transport)
	if tr == nil || tr.TLSClientConfig == nil || tr.TLSClientConfig.InsecureSkipVerify != true {
		t.Fatalf("expected InsecureSkipVerify=true for insecure mode")
	}
	// tls1.2: expect Min=Max=TLS1.2
	h12 := &Httpc{TlsConfig: &tls.Config{MinVersion: tls.VersionTLS12, MaxVersion: tls.VersionTLS12}}
	c12 := h12.New()
	tr, _ = c12.GetClient().Transport.(*http.Transport)
	if tr == nil || tr.TLSClientConfig == nil {
		t.Fatalf("expected TLSClientConfig for tls1.2 mode")
	}
	if tr.TLSClientConfig.MinVersion != tls.VersionTLS12 || tr.TLSClientConfig.MaxVersion != tls.VersionTLS12 {
		t.Fatalf("expected TLS1.2 only, got Min=%v Max=%v", tr.TLSClientConfig.MinVersion, tr.TLSClientConfig.MaxVersion)
	}
	// tls1.3: expect Min=Max=TLS1.3
	h13 := &Httpc{TlsConfig: &tls.Config{MinVersion: tls.VersionTLS13, MaxVersion: tls.VersionTLS13}}
	c13 := h13.New()
	tr, _ = c13.GetClient().Transport.(*http.Transport)
	if tr == nil || tr.TLSClientConfig == nil {
		t.Fatalf("expected TLSClientConfig for tls1.3 mode")
	}
	if tr.TLSClientConfig.MinVersion != tls.VersionTLS13 || tr.TLSClientConfig.MaxVersion != tls.VersionTLS13 {
		t.Fatalf("expected TLS1.3 only, got Min=%v Max=%v", tr.TLSClientConfig.MinVersion, tr.TLSClientConfig.MaxVersion)
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
	if code, err := doGet(t, context.Background(), srv.URL, nil); err != nil || code != 204 {
		t.Fatalf("default client to http server expected 204, got code=%d err=%v", code, err)
	}
}
