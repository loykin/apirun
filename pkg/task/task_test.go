package task

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
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
			Env:  Env{EnvMap: map[string]string{"AUTH": "token-123"}},
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

func TestTask_DownExecute_NotImplemented(t *testing.T) {
	task := Task{}
	res, err := task.DownExecute(context.Background(), http.MethodDelete, "http://example.invalid")
	if err == nil {
		t.Fatalf("expected not implemented error, got nil (res=%+v)", res)
	}
}
