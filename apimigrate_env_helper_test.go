package apimigrate

import (
	"context"
	"testing"
)

// Test struct-based Auth acquires a token via registered provider
func TestAuth_Acquire_StoresInAuth(t *testing.T) {
	// Register a dummy provider that returns a fixed token value
	RegisterAuthProvider("dummy", func(spec map[string]interface{}) (AuthMethod, error) {
		return dummyMethodEnvHelper{}, nil
	})

	ctx := context.Background()
	a := &Auth{Type: "dummy", Name: "demo", Methods: map[string]MethodConfig{"dummy": NewAuthSpecFromMap(map[string]interface{}{})}}
	v, err := a.Acquire(ctx, nil)
	if err != nil {
		t.Fatalf("Acquire error: %v", err)
	}
	if v != "Bearer unit-token" {
		t.Fatalf("unexpected token value: %q", v)
	}
}

type dummyMethodEnvHelper struct{}

func (d dummyMethodEnvHelper) Acquire(_ context.Context) (string, error) {
	return "Bearer unit-token", nil
}
