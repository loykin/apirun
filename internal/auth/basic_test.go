package auth

import (
	"context"
	"strings"
	"testing"
)

func TestAcquireToken_Basic_Success_DefaultHeader(t *testing.T) {
	spec := map[string]interface{}{
		"name":     "basic",
		"username": "alice",
		"password": "secret",
	}
	h, v, name, err := AcquireFromMap(context.Background(), "basic", spec)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.EqualFold(h, "authorization") {
		t.Fatalf("expected default Authorization header, got %s", h)
	}
	if name != "basic" {
		t.Fatalf("expected name 'basic', got %q", name)
	}
	expected := "Basic YWxpY2U6c2VjcmV0" // base64("alice:secret")
	if v != expected {
		t.Fatalf("expected %q, got %q", expected, v)
	}
}

func TestAcquireToken_Basic_CustomHeader(t *testing.T) {
	spec := map[string]interface{}{
		"name":     "basic",
		"username": "bob",
		"password": "p@ss",
		"header":   "X-Auth",
	}
	h, v, _, err := AcquireFromMap(context.Background(), "basic", spec)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if h != "X-Auth" {
		t.Fatalf("expected custom header, got %s", h)
	}
	expected := "Basic Ym9iOnBAc3M=" // base64("bob:p@ss")
	if v != expected {
		t.Fatalf("expected %q, got %q", expected, v)
	}
}

func TestAcquireToken_Basic_MissingCredentials_Error(t *testing.T) {
	cases := []map[string]interface{}{
		{"name": "basic", "username": "", "password": "x"},
		{"name": "basic", "username": "x", "password": ""},
	}
	for i, spec := range cases {
		_, _, _, err := AcquireFromMap(context.Background(), "basic", spec)
		if err == nil {
			t.Fatalf("case %d: expected error for missing credentials", i)
		}
		if !strings.Contains(err.Error(), "basic:") {
			t.Fatalf("case %d: unexpected error: %v", i, err)
		}
	}
}
