package auth

import (
	"context"
	"encoding/base64"
	"errors"
	"testing"

	"github.com/loykin/apimigrate/pkg/env"
)

func TestAuth_Acquire_Errors(t *testing.T) {
	var a *Auth
	if v, err := a.Acquire(context.Background(), nil); err != nil || v != "" {
		t.Fatalf("nil auth should be no-op: v=%q err=%v", v, err)
	}
	// missing type
	a = &Auth{}
	if _, err := a.Acquire(context.Background(), nil); err == nil {
		t.Fatalf("expected error for missing type")
	}
	// missing methods
	a = &Auth{Type: "basic"}
	if _, err := a.Acquire(context.Background(), nil); err == nil {
		t.Fatalf("expected error for missing methods")
	}
}

func TestAuth_Acquire_BasicProvider(t *testing.T) {
	ctx := context.Background()
	// basic provider is registered via internal/auth/registry.go init
	a := &Auth{Type: "basic", Name: "b", Methods: NewAuthSpecFromMap(map[string]interface{}{"username": "u", "password": "p"})}
	v, err := a.Acquire(ctx, nil)
	if err != nil {
		t.Fatalf("Acquire error: %v", err)
	}
	exp := base64.StdEncoding.EncodeToString([]byte("u:p"))
	if v != exp {
		t.Fatalf("unexpected token: got %q want %q", v, exp)
	}
}

func TestAuth_Acquire_RendersFromEnv(t *testing.T) {
	ctx := context.Background()
	base := env.Env{Global: env.FromStringMap(map[string]string{"user": "alice", "pass": "wonder"})}
	// Use basic provider but with templates pulling from env
	a := &Auth{Type: "basic", Name: "b", Methods: NewAuthSpecFromMap(map[string]interface{}{
		"username": "{{.env.user}}",
		"password": "{{.env.pass}}",
	})}
	v, err := a.Acquire(ctx, &base)
	if err != nil {
		t.Fatalf("Acquire error: %v", err)
	}
	exp := base64.StdEncoding.EncodeToString([]byte("alice:wonder"))
	if v != exp {
		t.Fatalf("unexpected token: got %q want %q", v, exp)
	}
}

// Negative test to ensure provider errors bubble up

type errMethod struct{}

func (e errMethod) Acquire(_ context.Context) (string, error) { return "", errors.New("boom") }

func TestAuth_Acquire_ProviderError(t *testing.T) {
	// temporarily register a failing provider
	Register("failing", func(spec map[string]interface{}) (Method, error) {
		return errMethod{}, nil
	})
	a := &Auth{Type: "failing", Methods: NewAuthSpecFromMap(map[string]interface{}{})}
	if _, err := a.Acquire(context.Background(), nil); err == nil {
		t.Fatalf("expected provider error")
	}
}
