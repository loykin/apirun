package task

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/loykin/apimigrate/pkg/env"
)

// Verifies Up.Execute success path with JSON body detection and env extraction
func TestUp_Execute_SuccessAndExtractEnv(t *testing.T) {
	// Mock server that validates header/query and returns JSON array for extraction
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", r.Method)
		}
		if r.Header.Get("jjj") != "bcd" {
			w.WriteHeader(401)
			return
		}
		if r.URL.Query().Get("abc") != "adsf" {
			w.WriteHeader(400)
			return
		}
		// Should set JSON header and body should be valid JSON
		if ct := r.Header.Get("Content-Type"); ct != "application/json" {
			t.Fatalf("expected Content-Type application/json, got %s", ct)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`[{"abc":"value_from_response"}]`))
	}))
	defer srv.Close()

	up := UpSpec{
		Name: "startup",
		Env:  env.Env{Local: map[string]string{"ABC": "ggg"}},
		Request: RequestSpec{
			Headers: []Header{{Name: "jjj", Value: "bcd"}},
			Queries: []Query{{Name: "abc", Value: "adsf"}},
			Body:    `{"abc": "{{.ABC}}"}`,
		},
		Response: ResponseSpec{
			ResultCode: []string{"200"},
			EnvFrom:    map[string]string{"abc": "$[0].abc"},
		},
	}

	res, err := up.Execute(context.Background(), http.MethodPost, srv.URL)
	if err != nil {
		t.Fatalf("expected success, got err: %v", err)
	}
	if res.StatusCode != 200 {
		t.Fatalf("expected status 200, got %d", res.StatusCode)
	}
	if res.ExtractedEnv["abc"] != "value_from_response" {
		t.Fatalf("expected extracted abc to be value_from_response, got %q", res.ExtractedEnv["abc"])
	}
}

// Verifies Up.Execute returns error on status mismatch when ResultCode restricts allowed statuses
func TestUp_Execute_StatusFailure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(503)
		_, _ = w.Write([]byte(`{"error":"down"}`))
	}))
	defer srv.Close()

	up := UpSpec{
		Name:    "startup",
		Env:     env.Env{},
		Request: RequestSpec{},
		Response: ResponseSpec{
			ResultCode: []string{"200"},
		},
	}

	res, err := up.Execute(context.Background(), http.MethodGet, srv.URL)
	if err == nil {
		t.Fatalf("expected error due to status mismatch, got nil")
	}
	if res == nil || res.StatusCode != 503 {
		t.Fatalf("expected result with status 503, got %+v", res)
	}
}

// Verifies unsupported methods are rejected
func TestUp_Execute_UnsupportedMethod(t *testing.T) {
	up := UpSpec{Request: RequestSpec{}, Response: ResponseSpec{}}
	res, err := up.Execute(context.Background(), "TRACE", "http://example.invalid")
	if err == nil {
		t.Fatalf("expected error for unsupported method, got nil (res=%+v)", res)
	}
}
