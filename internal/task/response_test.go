package task

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/loykin/apimigrate/internal/env"
)

// Verify that when ResultCode is not specified, any status is accepted as success.
func TestExecuteUp_NoResultCode_AllSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(503)
		_, _ = w.Write([]byte(`{"status":"maintenance"}`))
	}))
	defer srv.Close()

	up := UpSpec{
		Name:    "startup",
		Env:     env.Env{},
		Request: RequestSpec{},
		Response: ResponseSpec{
			ResultCode: nil, // no codes specified
		},
	}

	res, err := up.Execute(context.Background(), http.MethodGet, srv.URL)
	if err != nil {
		t.Fatalf("expected success with no result_code restrictions, got err: %v", err)
	}
	if res == nil {
		t.Fatalf("expected non-nil result")
	}
	if res.StatusCode != 503 {
		t.Fatalf("expected status 503, got %d", res.StatusCode)
	}
}
