package apimigrate

import (
	"context"
	"testing"
)

// TestAcquireAuthAndSetEnv_StoresInAuth verifies the helper acquires a token and stores under .auth[name].
func TestAcquireAuthAndSetEnv_StoresInAuth(t *testing.T) {
	// Register a dummy provider that returns a fixed token value
	RegisterAuthProvider("dummy", func(spec map[string]interface{}) (AuthMethod, error) {
		return dummyMethodEnvHelper{}, nil
	})

	ctx := context.Background()
	base := Env{Global: map[string]string{"pre": "x"}}
	v, err := AcquireAuthAndSetEnv(ctx, "dummy", "demo", NewAuthSpecFromMap(map[string]interface{}{}), &base)
	if err != nil {
		t.Fatalf("AcquireAuthAndSetEnv error: %v", err)
	}
	if v != "Bearer unit-token" {
		t.Fatalf("unexpected token value: %q", v)
	}
	if base.Auth["demo"] != v {
		t.Fatalf("auth token not stored under name: got %q", base.Auth["demo"])
	}
}

type dummyMethodEnvHelper struct{}

func (d dummyMethodEnvHelper) Acquire(_ context.Context) (string, error) {
	return "Bearer unit-token", nil
}
