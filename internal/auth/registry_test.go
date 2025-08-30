package auth

import (
	"context"
	"fmt"
	"testing"
)

type testMethod struct {
	value string
}

// Ensure ctx is non-nil: return error if nil
func (m testMethod) Acquire(ctx context.Context) (string, error) {
	if ctx == nil {
		return "", fmt.Errorf("nil context passed to Acquire")
	}
	return m.value, nil
}

// custom factory helper
func makeFactory(_ string, _ string, value string) Factory {
	return func(spec map[string]interface{}) (Method, error) {
		if v, ok := spec["value"].(string); ok && v != "" {
			value = v
		}
		return testMethod{value: value}, nil
	}
}

func TestRegistry_RegisterAndAcquire_CustomProvider(t *testing.T) {
	ClearTokens()
	Register("UnitTestDemo", makeFactory("demo", "X-Demo", "ok"))

	v, err := AcquireAndStoreWithName(context.TODO(), "unittestdemo", "demo", map[string]interface{}{"value": "val"})
	if err != nil {
		t.Fatalf("AcquireAndStoreWithName err: %v", err)
	}
	if v != "val" {
		t.Fatalf("unexpected acquire: v=%q", v)
	}
	// stored under provided name
	sh, sv, ok := GetToken("demo")
	if !ok || sh == "" || sv != v {
		t.Fatalf("expected token stored as 'demo': ok=%v h=%q v=%q", ok, sh, sv)
	}
}

func TestRegistry_AcquireAndStoreWithName_StoresToken(t *testing.T) {
	ClearTokens()
	Register("UnitTestStore", makeFactory("store", "Authorization", "Bearer 123"))

	v, err := AcquireAndStoreWithName(context.Background(), "unitteststore", "store", map[string]interface{}{})
	if err != nil {
		t.Fatalf("AcquireAndStoreWithName err: %v", err)
	}
	if v == "" {
		t.Fatalf("expected non-empty value, got %q", v)
	}
	gh, gv, ok := GetToken("store")
	if !ok || gh == "" || gv != v {
		t.Fatalf("expected token stored: ok=%v gh=%q gv=%q; want v=%q", ok, gh, gv, v)
	}
}

func TestRegistry_UnsupportedType_ReturnsError(t *testing.T) {
	if _, err := AcquireAndStoreWithName(context.Background(), "does-not-exist", "any", nil); err == nil {
		t.Fatalf("expected error for unsupported provider, got nil")
	}
}

func TestRegistry_Register_IgnoresEmptyOrNil(t *testing.T) {
	// empty type
	Register("", nil)
	if _, err := AcquireAndStoreWithName(context.Background(), "", "n", nil); err == nil {
		t.Fatalf("expected error for empty type after Register(\"\", nil)")
	}
}
