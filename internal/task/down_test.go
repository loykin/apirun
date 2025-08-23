package task

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

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
