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

type dummyMethod struct{ v string }

func (d dummyMethod) Acquire(_ context.Context) (string, error) { return d.v, nil }

func dummyFactoryOK(spec map[string]interface{}) (Method, error) {
	v, _ := spec["value"].(string)
	return dummyMethod{v: v}, nil
}

func TestRegister_IgnoresEmptyKeyAndNilFactory(t *testing.T) {
	// Capture current size
	before := len(providers)
	Register("", dummyFactoryOK)   // ignored due to empty key
	Register("  ", dummyFactoryOK) // ignored due to whitespace
	Register("demo", nil)          // ignored due to nil factory
	if len(providers) != before {
		t.Fatalf("providers size changed unexpectedly: before=%d after=%d", before, len(providers))
	}
}

func TestNormalizeKey_LowerTrim(t *testing.T) {
	if got := normalizeKey("  OAuth2  "); got != "oauth2" {
		t.Fatalf("normalizeKey mismatch: got %q want %q", got, "oauth2")
	}
}

func TestAcquireAndStoreWithName_UnsupportedTypeError(t *testing.T) {
	ClearTokens()
	if _, err := AcquireAndStoreWithName(context.Background(), "__nope__", "x", map[string]interface{}{}); err == nil {
		t.Fatal("expected error for unsupported provider type, got nil")
	}
}

func TestAcquireAndStoreWithName_StoresTokenAndRetrievable(t *testing.T) {
	ClearTokens()
	// Register a unique temporary provider key
	key := "dummy-registry-test"
	Register(key, dummyFactoryOK)
	v, err := AcquireAndStoreWithName(context.Background(), key, "logical", map[string]interface{}{"value": "tok123"})
	if err != nil {
		t.Fatalf("AcquireAndStoreWithName error: %v", err)
	}
	if v != "tok123" {
		t.Fatalf("unexpected token value: got %q want %q", v, "tok123")
	}
	// Ensure it was stored under the provided logical name with header Authorization
	h, sv, ok := GetToken("logical")
	if !ok || h != "Authorization" || sv != "tok123" {
		t.Fatalf("stored token mismatch: ok=%v header=%q val=%q", ok, h, sv)
	}
}

func TestAcquireAndStoreWithName_NilContextHandled(t *testing.T) {
	ClearTokens()
	key := "dummy-nilctx"
	Register(key, dummyFactoryOK)
	v, err := AcquireAndStoreWithName(context.TODO(), key, "nm", map[string]interface{}{"value": "x"})
	if err != nil || v != "x" {
		t.Fatalf("nil ctx path failed: v=%q err=%v", v, err)
	}
}
