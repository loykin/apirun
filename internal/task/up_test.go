package task

import (
	"context"
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/loykin/apirun/pkg/env"
)

func TestUp_Execute_OverrideMethodURL_ExtractEnv(t *testing.T) {
	// Server expects POST /create?q=ok with header and json body; returns 201 with id
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/create" {
			t.Fatalf("expected path /create, got %s", r.URL.Path)
		}
		if r.URL.Query().Get("q") != "ok" {
			t.Fatalf("expected query q=ok")
		}
		if r.Header.Get("X-Name") != "alice" {
			t.Fatalf("expected X-Name=alice, got %q", r.Header.Get("X-Name"))
		}
		w.WriteHeader(201)
		_, _ = w.Write([]byte(`{"id":"123","ok":true}`))
	}))
	defer srv.Close()

	u := Up{
		Name: "create",
		Env:  &env.Env{Local: env.FromStringMap(map[string]string{"name": "alice", "q": "ok"})},
		Request: RequestSpec{
			Method:  http.MethodPost,                  // should override provided method
			URL:     srv.URL + "/create?q={{.env.q}}", // should override provided URL
			Headers: []Header{{Name: "X-Name", Value: "{{.env.name}}"}},
			Body:    `{"x":"{{.env.name}}"}`,
		},
		Response: ResponseSpec{
			ResultCode: []string{"201"},
			EnvFrom:    map[string]string{"rid": "id"},
		},
	}

	res, err := u.Execute(context.Background(), http.MethodGet, "http://ignored")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if res == nil || res.StatusCode != 201 {
		t.Fatalf("expected status 201, got %+v", res)
	}
	if res.ExtractedEnv["rid"] != "123" {
		t.Fatalf("expected extracted rid=123, got %v", res.ExtractedEnv)
	}
}

func TestUp_TLS_Insecure_AllowsSelfSigned(t *testing.T) {
	// HTTPS server with self-signed cert
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	u := Up{
		Env: &env.Env{},
		Request: RequestSpec{
			Method: http.MethodGet,
			URL:    srv.URL,
		},
		Response: ResponseSpec{ResultCode: []string{"200"}},
	}
	// First, with default TLS (no insecure), request to self-signed server should fail
	SetTLSConfig(nil)
	if _, err := u.Execute(context.Background(), http.MethodGet, srv.URL); err == nil {
		t.Fatalf("expected TLS verification error without insecure, got nil")
	}
	// Now enable insecure and expect success
	SetTLSConfig(&tls.Config{InsecureSkipVerify: true})
	if res, err := u.Execute(context.Background(), http.MethodGet, srv.URL); err != nil || res == nil || res.StatusCode != 200 {
		t.Fatalf("expected success with insecure TLS, got res=%v err=%v", res, err)
	}
}

func TestUp_Execute_StatusNotAllowed_ReturnsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
		_, _ = w.Write([]byte(`{"err":true}`))
	}))
	defer srv.Close()

	u := Up{
		Env: &env.Env{},
		Request: RequestSpec{
			Method: http.MethodGet,
			URL:    srv.URL,
		},
		Response: ResponseSpec{ResultCode: []string{"200"}},
	}

	res, err := u.Execute(context.Background(), http.MethodGet, srv.URL)
	if err == nil {
		t.Fatalf("expected error due to disallowed status, got nil")
	}
	if res == nil || res.StatusCode != 500 {
		t.Fatalf("expected ExecResult with status 500, got %+v", res)
	}
}

// Verify env_missing=fail returns error when a mapped key is absent, while default skip does not.
func TestUp_Execute_EnvMissingPolicy(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{"present":"ok"}`))
	}))
	defer srv.Close()

	tRun := func(name string, envMissing string, wantErr bool) {
		t.Run(name, func(t *testing.T) {
			u := Up{
				Env:     &env.Env{},
				Request: RequestSpec{Method: http.MethodGet, URL: srv.URL},
				Response: ResponseSpec{
					ResultCode: []string{"200"},
					EnvFrom:    map[string]string{"a": "present", "b": "missing"},
					EnvMissing: envMissing,
				},
			}
			res, err := u.Execute(context.Background(), http.MethodGet, srv.URL)
			if wantErr {
				if err == nil {
					t.Fatalf("expected error due to missing env var, got nil")
				}
				if res == nil || res.StatusCode != 200 {
					t.Fatalf("expected ExecResult with status 200, got %+v", res)
				}
				if res.ExtractedEnv["a"] != "ok" {
					t.Fatalf("expected extracted a=ok, got %+v", res.ExtractedEnv)
				}
			} else {
				if err != nil {
					t.Fatalf("unexpected error with skip policy: %v", err)
				}
				if res == nil || res.StatusCode != 200 {
					t.Fatalf("expected 200, got %+v", res)
				}
				if res.ExtractedEnv["a"] != "ok" {
					t.Fatalf("expected extracted a=ok, got %+v", res.ExtractedEnv)
				}
				if _, ok := res.ExtractedEnv["b"]; ok {
					t.Fatalf("did not expect b to be present in extracted env")
				}
			}
		})
	}

	tRun("fail-policy", "fail", true)
	tRun("skip-default", "", false)
}
