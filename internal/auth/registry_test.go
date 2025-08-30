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
	vv, ok := GetToken("demo")
	if !ok || vv != v {
		t.Fatalf("expected token stored as 'demo': ok=%v v=%q want=%q", ok, vv, v)
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
	vv, ok := GetToken("store")
	if !ok || vv != v {
		t.Fatalf("expected token stored: ok=%v v=%q; want v=%q", ok, vv, v)
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
	// Ensure it was stored under the provided logical name
	vv, ok := GetToken("logical")
	if !ok || vv != "tok123" {
		t.Fatalf("stored token mismatch: ok=%v val=%q", ok, vv)
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

// Ensure built-in providers are registered by init() and factories can build Methods.
func TestRegistry_BuiltinsRegisteredAndBuildable(t *testing.T) {
	// oauth2
	f, ok := providers[normalizeKey("oauth2")]
	if !ok || f == nil {
		t.Fatal("oauth2 provider not registered in init")
	}
	// Build an implicit grant method to avoid network
	m, err := f(map[string]interface{}{
		"grant_type": "implicit",
		"grant_config": map[string]interface{}{
			"client_id":    "cid",
			"redirect_url": "http://localhost/redirect",
			"auth_url":     "http://auth.example/authorize",
		},
	})
	if err != nil || m == nil {
		t.Fatalf("oauth2 factory build failed: m=%#v err=%v", m, err)
	}

	// basic
	f2, ok := providers[normalizeKey("basic")]
	if !ok || f2 == nil {
		t.Fatal("basic provider not registered in init")
	}
	m2, err := f2(map[string]interface{}{"username": "u", "password": "p"})
	if err != nil || m2 == nil {
		t.Fatalf("basic factory build failed: m=%#v err=%v", m2, err)
	}
	if _, err := m2.Acquire(context.Background()); err != nil {
		t.Fatalf("basic acquire failed unexpectedly: %v", err)
	}

	// pocketbase
	f3, ok := providers[normalizeKey("pocketbase")]
	if !ok || f3 == nil {
		t.Fatal("pocketbase provider not registered in init")
	}
	// Build method; do not call Acquire to avoid network
	m3, err := f3(map[string]interface{}{"base_url": "http://example", "email": "a@b.c", "password": "x"})
	if err != nil || m3 == nil {
		t.Fatalf("pocketbase factory build failed: m=%#v err=%v", m3, err)
	}
}
