package pocketbase_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/loykin/apimigrate/internal/auth"
)

func TestPocketBase_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/admins/auth-with-password" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"token":"pbtoken"}`))
	}))
	defer srv.Close()

	spec := map[string]interface{}{
		"name":     "pocket",
		"base_url": srv.URL,
		"email":    "admin@example.com",
		"password": "secret",
	}

	h, v, _, err := auth.AcquireFromMap(context.Background(), "pocketbase", spec)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if h == "" || v == "" {
		t.Fatalf("expected non-empty header/value, got %q %q", h, v)
	}
	if v != "pbtoken" {
		t.Fatalf("expected token 'pbtoken', got %q", v)
	}
}

func TestPocketBase_Non2xx_Error(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(401)
		_, _ = w.Write([]byte(`{"error":"unauthorized"}`))
	}))
	defer srv.Close()

	spec := map[string]interface{}{
		"name":     "pocket",
		"base_url": srv.URL,
		"email":    "admin@example.com",
		"password": "secret",
	}

	_, _, _, err := auth.AcquireFromMap(context.Background(), "pocketbase", spec)
	if err == nil {
		t.Fatalf("expected error for non-2xx response")
	}
}
