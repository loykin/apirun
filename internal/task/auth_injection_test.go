package task

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/loykin/apimigrate/internal/auth"
	"github.com/loykin/apimigrate/internal/env"
)

func TestRequest_Render_InjectsTokenFromAuthStore_Authorization(t *testing.T) {
	auth.ClearTokens()
	auth.SetToken("keycloak", "Authorization", "Bearer XYZ")

	// server checks Authorization header exists
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer XYZ" {
			t.Fatalf("expected Authorization Bearer XYZ, got %q", r.Header.Get("Authorization"))
		}
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`[]`))
	}))
	defer srv.Close()

	up := Up{
		Env:      env.Env{},
		Request:  RequestSpec{AuthName: "keycloak"},
		Response: ResponseSpec{ResultCode: []string{"200"}},
	}

	if _, err := up.Execute(context.Background(), http.MethodGet, srv.URL); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRequest_Render_InjectsCustomHeaderFromAuthStore(t *testing.T) {
	auth.ClearTokens()
	auth.SetToken("pocketbase", "X-Api-Key", "abc123")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Api-Key") != "abc123" {
			t.Fatalf("expected X-Api-Key header injected, got %q", r.Header.Get("X-Api-Key"))
		}
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`[]`))
	}))
	defer srv.Close()

	up := Up{
		Request:  RequestSpec{AuthName: "pocketbase"},
		Response: ResponseSpec{ResultCode: []string{"200"}},
	}

	if _, err := up.Execute(context.Background(), http.MethodGet, srv.URL); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRequest_Render_DoesNotOverrideExistingHeaderFromAuthStore(t *testing.T) {
	auth.ClearTokens()
	auth.SetToken("svc", "Authorization", "Bearer should-not-apply")

	// Existing header must be preserved
	req := RequestSpec{
		AuthName: "svc",
		Headers:  []Header{{Name: "Authorization", Value: "Bearer preset"}},
	}

	hdrs, _, _ := req.Render(env.Env{})
	if got := hdrs["Authorization"]; got != "Bearer preset" {
		t.Fatalf("expected header preserved, got %q", got)
	}
}
