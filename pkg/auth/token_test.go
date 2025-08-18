package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestTokenStore_SetGetClear(t *testing.T) {
	ClearTokens()
	SetToken("KeyCloak", "Authorization", "Bearer XYZ")

	h, v, ok := GetToken("keycloak")
	if !ok {
		t.Fatalf("expected token to exist")
	}
	if !strings.EqualFold(h, "authorization") || v != "Bearer XYZ" {
		t.Fatalf("unexpected header/value: %s %q", h, v)
	}

	// Clear and verify removed
	ClearTokens()
	if _, _, ok := GetToken("keycloak"); ok {
		t.Fatalf("expected token to be cleared")
	}
}

func TestTokenStore_DoesNotStoreInvalid(t *testing.T) {
	ClearTokens()
	SetToken("", "Authorization", "x")
	SetToken("svc", "", "x")
	SetToken("svc2", "Authorization", "")

	if _, _, ok := GetToken(""); ok {
		t.Fatalf("empty name should not be stored")
	}
	if _, _, ok := GetToken("svc"); ok {
		t.Fatalf("missing header should not be stored")
	}
	if _, _, ok := GetToken("svc2"); ok {
		t.Fatalf("missing value should not be stored")
	}
}

func TestAcquireAndStore_StoresResult_PocketBase(t *testing.T) {
	ClearTokens()
	// Mock PocketBase endpoint
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/users/auth-with-password" {
			w.WriteHeader(404)
			return
		}
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{"token":"abc123"}`))
	}))
	defer srv.Close()

	cfg := Config{Provider: ProviderConfig{
		Name:        "pocketbase",
		BaseURL:     srv.URL,
		Identity:    "user@example.com",
		Password:    "pw",
		TokenHeader: "", // default Authorization
	}}

	h, v, err := AcquireAndStore(context.Background(), cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.EqualFold(h, "authorization") || v != "Bearer abc123" {
		t.Fatalf("unexpected returned header/token: %s %q", h, v)
	}

	// Should be stored under provider name
	sh, sv, ok := GetToken("pocketbase")
	if !ok {
		t.Fatalf("expected token stored under name 'pocketbase'")
	}
	if !strings.EqualFold(sh, "authorization") || sv != "Bearer abc123" {
		t.Fatalf("unexpected stored header/token: %s %q", sh, sv)
	}
}
