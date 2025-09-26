package basic_test

import (
	"context"
	"strings"
	"testing"

	"github.com/loykin/apirun/internal/auth"
	"github.com/loykin/apirun/internal/auth/basic"
)

func TestAcquireToken_Basic_Success_DefaultHeader(t *testing.T) {
	spec := map[string]interface{}{
		"username": "alice",
		"password": "secret",
	}
	v, err := auth.AcquireAndStoreWithName(context.Background(), "basic", spec)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := "YWxpY2U6c2VjcmV0" // base64("alice:secret") without scheme
	if v != expected {
		t.Fatalf("expected %q, got %q", expected, v)
	}
}

func TestAcquireToken_Basic_CustomHeader(t *testing.T) {
	spec := map[string]interface{}{
		"username": "bob",
		"password": "p@ss",
		"header":   "X-Auth",
	}
	v, err := auth.AcquireAndStoreWithName(context.Background(), "basic", spec)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := "Ym9iOnBAc3M=" // base64("bob:p@ss") without scheme
	if v != expected {
		t.Fatalf("expected %q, got %q", expected, v)
	}
}

func TestAcquireToken_Basic_MissingCredentials_Error(t *testing.T) {
	cases := []map[string]interface{}{
		{"username": "", "password": "x"},
		{"username": "x", "password": ""},
	}
	for i, spec := range cases {
		_, err := auth.AcquireAndStoreWithName(context.Background(), "basic", spec)
		if err == nil {
			t.Fatalf("case %d: expected error for missing credentials", i)
		}
		if !strings.Contains(err.Error(), "basic:") {
			t.Fatalf("case %d: unexpected error: %v", i, err)
		}
	}
}

func TestInternalBasicConfig_ToMap(t *testing.T) {
	c := basic.Config{Username: "u", Password: "p"}
	m := c.ToMap()
	if m["username"] != "u" || m["password"] != "p" {
		t.Fatalf("basic.Config.ToMap mismatch: %+v", m)
	}
}
