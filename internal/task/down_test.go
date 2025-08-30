package task

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/loykin/apimigrate/internal/auth"
	"github.com/loykin/apimigrate/internal/env"
)

func TestDown_Execute_WithFindAndTemplatingAndAuthFromEnv(t *testing.T) {
	calls := struct{ find, del int }{}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			calls.find++
			if !strings.Contains(r.URL.RawQuery, "name=bob") {
				t.Fatalf("find query does not contain name=bob: %s", r.URL.RawQuery)
			}
			w.WriteHeader(200)
			_, _ = w.Write([]byte(`[{"id":"42"}]`))
		case http.MethodDelete:
			calls.del++
			if r.URL.Path != "/items/42" {
				t.Fatalf("expected DELETE /items/42, got %s", r.URL.Path)
			}
			if r.Header.Get("Authorization") != "Bearer abc" {
				t.Fatalf("expected Authorization from env, got %q", r.Header.Get("Authorization"))
			}
			if r.Header.Get("X-Del") != "yes" {
				t.Fatalf("expected X-Del=yes, got %q", r.Header.Get("X-Del"))
			}
			if r.URL.Query().Get("reason") != "cleanup" {
				t.Fatalf("expected reason=cleanup, got %q", r.URL.Query().Get("reason"))
			}
			w.WriteHeader(204)
		default:
			t.Fatalf("unexpected method %s", r.Method)
		}
	}))
	defer srv.Close()

	d := Down{
		Name: "teardown",
		Auth: "token", // will pick from env TOKEN
		Env:  env.Env{Local: map[string]string{"TOKEN": "Bearer abc", "flag": "yes", "reason": "cleanup", "name": "bob"}},
		Find: &FindSpec{
			Request: RequestSpec{
				Method: http.MethodGet,
				URL:    srv.URL + "/items?name={{.name}}",
			},
			Response: ResponseSpec{
				ResultCode: []string{"200"},
				EnvFrom:    map[string]string{"user_id": "0.id"},
			},
		},
		Method: http.MethodDelete,
		URL:    srv.URL + "/items/{{.user_id}}",
		Headers: []Header{
			{Name: "X-Del", Value: "{{.flag}}"},
		},
		Queries: []Query{{Name: "reason", Value: "{{.reason}}"}},
		Body:    "{}",
	}

	res, err := d.Execute(context.Background())
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if res == nil || res.StatusCode != 204 {
		t.Fatalf("expected status 204, got %+v", res)
	}
	if calls.find != 1 || calls.del != 1 {
		t.Fatalf("expected one find and one delete, got find=%d del=%d", calls.find, calls.del)
	}
}

func TestDown_Execute_FinalNon2xx_ReturnsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			w.WriteHeader(200)
			_, _ = w.Write([]byte(`[]`))
			return
		}
		w.WriteHeader(500)
	}))
	defer srv.Close()

	d := Down{
		Env:    env.Env{},
		Method: http.MethodDelete,
		URL:    srv.URL + "/x",
	}

	res, err := d.Execute(context.Background())
	if err == nil {
		t.Fatalf("expected error for non-2xx final status, got nil")
	}
	if res == nil || res.StatusCode != 500 {
		t.Fatalf("expected ExecResult with status 500, got %+v", res)
	}
}

func TestDown_Execute_DoesNotOverrideExplicitAuthorizationHeader(t *testing.T) {
	auth.ClearTokens()
	auth.SetToken("a1", "Authorization", "Bearer NEWTOKEN")
	calls := 0

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if got := r.Header.Get("Authorization"); got != "Basic OLD" {
			t.Fatalf("expected explicit Authorization to be preserved, got %q", got)
		}
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	d := Down{
		Name:   "no-override",
		Auth:   "a1",
		Env:    env.Env{Local: map[string]string{}},
		Method: http.MethodDelete,
		URL:    srv.URL + "/x",
		Headers: []Header{
			{Name: "Authorization", Value: "Basic OLD"},
		},
	}
	res, err := d.Execute(context.Background())
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if res == nil || res.StatusCode != 200 {
		t.Fatalf("expected 200, got %+v", res)
	}
	if calls != 1 {
		t.Fatalf("expected 1 call, got %d", calls)
	}
}

func TestDown_Execute_InjectsCustomHeaderFromTokenRegistry(t *testing.T) {
	auth.ClearTokens()
	auth.SetToken("my", "X-Api-Key", "abc123")
	calls := 0

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if got := r.Header.Get("X-Api-Key"); got != "abc123" {
			t.Fatalf("expected X-Api-Key header injected, got %q", got)
		}
		if got := r.Header.Get("Authorization"); got != "" {
			t.Fatalf("did not expect Authorization header, got %q", got)
		}
		w.WriteHeader(204)
	}))
	defer srv.Close()

	d := Down{
		Name:   "custom-header",
		Auth:   "my",
		Env:    env.Env{Local: map[string]string{}},
		Method: http.MethodDelete,
		URL:    srv.URL + "/x",
	}
	res, err := d.Execute(context.Background())
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if res == nil || res.StatusCode != 204 {
		t.Fatalf("expected 204, got %+v", res)
	}
	if calls != 1 {
		t.Fatalf("expected 1 call, got %d", calls)
	}
}

func TestDown_Find_ValidationFailure_ReturnsExecResultAndError(t *testing.T) {
	calls := struct{ find, del int }{}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			calls.find++
			w.WriteHeader(200)
			_, _ = w.Write([]byte(`[]`))
			return
		}
		calls.del++
		w.WriteHeader(200)
	}))
	defer srv.Close()

	d := Down{
		Name: "with-find",
		Env:  env.Env{Local: map[string]string{}},
		Find: &FindSpec{
			Request: RequestSpec{
				Method: http.MethodGet,
				URL:    srv.URL + "/search",
			},
			Response: ResponseSpec{
				// Intentionally require 404 so validation fails on 200
				ResultCode: []string{"404"},
			},
		},
		Method: http.MethodDelete,
		URL:    srv.URL + "/will-not-run",
	}

	res, err := d.Execute(context.Background())
	if err == nil {
		t.Fatalf("expected validation error from find, got nil")
	}
	if res == nil || res.StatusCode != 200 {
		t.Fatalf("expected ExecResult with find status 200, got %+v", res)
	}
	if calls.find != 1 || calls.del != 0 {
		t.Fatalf("expected find=1 del=0, got find=%d del=%d", calls.find, calls.del)
	}
}

func TestDown_Execute_JSONBodySetsContentType(t *testing.T) {
	calls := 0
	var gotCT string
	var gotBody string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		gotCT = r.Header.Get("Content-Type")
		b, _ := io.ReadAll(r.Body)
		_ = r.Body.Close()
		gotBody = string(b)
		w.WriteHeader(200)
	}))
	defer srv.Close()

	d := Down{
		Env:    env.Env{Local: map[string]string{"x": "1"}},
		Method: http.MethodDelete,
		URL:    srv.URL + "/json",
		Body:   `{"a":{{.x}}}`,
	}
	res, err := d.Execute(context.Background())
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if res == nil || res.StatusCode != 200 {
		t.Fatalf("expected 200, got %+v", res)
	}
	if gotCT != "application/json" {
		t.Fatalf("expected Content-Type application/json, got %q", gotCT)
	}
	if gotBody != "{\"a\":1}" {
		t.Fatalf("unexpected body: %q", gotBody)
	}
}

func TestExecByMethod_SupportedAndUnsupported(t *testing.T) {
	// Set up a server that responds 201 to all methods to distinguish from default 200
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(201)
	}))
	defer srv.Close()

	req := buildRequest(context.Background(), map[string]string{}, map[string]string{}, "")
	// Supported methods
	cases := []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete}
	for _, m := range cases {
		resp, err := execByMethod(req, m, srv.URL)
		if err != nil {
			t.Fatalf("%s: unexpected err: %v", m, err)
		}
		if resp.StatusCode() != 201 {
			t.Fatalf("%s: expected 201, got %d", m, resp.StatusCode())
		}
	}
	// Unsupported method (HEAD is not implemented in switch)
	if _, err := execByMethod(req, http.MethodHead, srv.URL); err == nil {
		t.Fatalf("HEAD: expected error for unsupported method")
	}
}
