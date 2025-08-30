package pocketbase_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/loykin/apimigrate/internal/auth"
	"github.com/loykin/apimigrate/internal/auth/pocketbase"
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
		"base_url": srv.URL,
		"email":    "admin@example.com",
		"password": "secret",
	}

	v, err := auth.AcquireAndStoreWithName(context.Background(), "pocketbase", "pocket", spec)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	sh, sv, ok := auth.GetToken("pocket")
	if !ok || sh == "" || sv == "" || sv != v {
		t.Fatalf("expected stored token, got ok=%v header=%q val=%q", ok, sh, sv)
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
		"base_url": srv.URL,
		"email":    "admin@example.com",
		"password": "secret",
	}

	_, err := auth.AcquireAndStoreWithName(context.Background(), "pocketbase", "pocket", spec)
	if err == nil {
		t.Fatalf("expected error for non-2xx response")
	}
}

func TestInternalPocketBaseConfig_ToMap(t *testing.T) {
	c := pocketbase.Config{BaseURL: "b", Email: "e", Password: "p"}
	m := c.ToMap()
	if m["base_url"] != "b" || m["email"] != "e" || m["password"] != "p" {
		t.Fatalf("pocketbase.Config.ToMap mismatch: %+v", m)
	}
}
