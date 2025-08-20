package task

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/loykin/apimigrate/pkg/env"
)

func TestTask_UpExecute_Success(t *testing.T) {
	// Mock server for a simple success path
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", r.Method)
		}
		if r.Header.Get("X-Auth") != "token-123" {
			t.Fatalf("missing header X-Auth")
		}
		if r.URL.Query().Get("q") != "v" {
			t.Fatalf("missing query q=v")
		}
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`[]`))
	}))
	defer srv.Close()

	tsk := Task{
		Up: Up{
			Name: "startup",
			Env:  env.Env{Local: map[string]string{"AUTH": "token-123"}},
			Request: RequestSpec{
				AuthName: "auth", // will set Authorization only if specified, but we keep a custom header too
				Headers:  []Header{{Name: "X-Auth", Value: "{{.AUTH}}"}},
				Queries:  []Query{{Name: "q", Value: "v"}},
				Body:     `{"ok":true}`,
			},
			Response: ResponseSpec{ResultCode: []string{"200"}},
		},
	}

	res, err := tsk.UpExecute(context.Background(), http.MethodPost, srv.URL)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if res == nil || res.StatusCode != 200 {
		t.Fatalf("expected status 200, got %+v", res)
	}
}

func TestTask_DownExecute_Success(t *testing.T) {
	// Mock server that accepts DELETE and records a header
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Fatalf("expected DELETE, got %s", r.Method)
		}
		if r.Header.Get("X-Del") != "yes" {
			t.Fatalf("missing header X-Del=yes")
		}
		w.WriteHeader(200)
	}))
	defer srv.Close()

	tsk := Task{Down: Down{
		Name:   "teardown",
		Env:    env.Env{Local: map[string]string{"flag": "yes"}},
		Method: http.MethodDelete,
		URL:    srv.URL,
		Headers: []Header{
			{Name: "X-Del", Value: "{{.flag}}"},
		},
	}}

	res, err := tsk.DownExecute(context.Background(), "", "")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if res == nil || res.StatusCode != 200 {
		t.Fatalf("expected status 200, got %+v", res)
	}
}
