package task

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/loykin/apimigrate/internal/env"
)

func TestRequest_Render_UsesAuthTokenFromEnvTemplate(t *testing.T) {
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
		Env:      env.Env{Local: map[string]string{"_auth_token": "Bearer XYZ"}},
		Request:  RequestSpec{Headers: []Header{{Name: "Authorization", Value: "{{._auth_token}}"}}},
		Response: ResponseSpec{ResultCode: []string{"200"}},
	}

	if _, err := up.Execute(context.Background(), http.MethodGet, srv.URL); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRequest_Render_DoesNotOverrideExistingHeader(t *testing.T) {
	// Existing header must be preserved (no auto-injection)
	req := RequestSpec{
		Headers: []Header{{Name: "Authorization", Value: "Bearer preset"}},
	}

	hdrs, _, _ := req.Render(env.Env{})
	if got := hdrs["Authorization"]; got != "Bearer preset" {
		t.Fatalf("expected header preserved, got %q", got)
	}
}
