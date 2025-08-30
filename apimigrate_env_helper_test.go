package apimigrate

import (
	"context"
	"testing"
)

// TestAcquireAuthAndSetEnv_SetsAuthToken verifies the helper acquires and injects _auth_token.
func TestAcquireAuthAndSetEnv_SetsAuthToken(t *testing.T) {
	// Register a dummy provider that returns a fixed token value
	RegisterAuthProvider("dummy", func(spec map[string]interface{}) (AuthMethod, error) {
		return dummyMethodEnvHelper{}, nil
	})

	ctx := context.Background()
	base := Env{Global: map[string]string{"pre": "x"}}
	v, err := AcquireAuthAndSetEnv(ctx, "dummy", "d1", map[string]interface{}{}, &base)
	if err != nil {
		t.Fatalf("AcquireAuthAndSetEnv error: %v", err)
	}
	if v != "Bearer unit-token" {
		t.Fatalf("unexpected token value: %q", v)
	}
	if base.Global["_auth_token"] != v {
		t.Fatalf("_auth_token not set correctly: %q", base.Global["_auth_token"])
	}
}

type dummyMethodEnvHelper struct{}

func (d dummyMethodEnvHelper) Acquire(_ context.Context) (string, error) {
	return "Bearer unit-token", nil
}
