package auth

import (
	"context"
	"fmt"
	"testing"
)

type testMethod struct {
	name   string
	header string
	value  string
}

func (m testMethod) Name() string { return m.name }

// Ensure ctx is non-nil: return error if nil (AcquireFromMap should prevent this)
func (m testMethod) Acquire(ctx context.Context) (string, string, error) {
	if ctx == nil {
		return "", "", fmt.Errorf("nil context passed to Acquire")
	}
	return m.header, m.value, nil
}

// custom factory helper
func makeFactory(name, header, value string) Factory {
	return func(spec map[string]interface{}) (Method, error) {
		// allow overrides from spec if provided
		if v, ok := spec["name"].(string); ok && v != "" {
			name = v
		}
		if v, ok := spec["header"].(string); ok && v != "" {
			header = v
		}
		if v, ok := spec["value"].(string); ok && v != "" {
			value = v
		}
		return testMethod{name: name, header: header, value: value}, nil
	}
}

func TestRegistry_RegisterAndAcquire_CustomProvider(t *testing.T) {
	ClearTokens()
	typ := "UnitTestDemo"
	Register(typ, makeFactory("demo", "X-Demo", "ok"))

	h, v, name, err := AcquireFromMap(nil, "unittestdemo", map[string]interface{}{"value": "val"})
	if err != nil {
		t.Fatalf("AcquireFromMap err: %v", err)
	}
	if h != "X-Demo" || v != "val" || name != "demo" {
		t.Fatalf("unexpected acquire: h=%q v=%q name=%q", h, v, name)
	}
}

func TestRegistry_AcquireAndStoreFromMap_StoresToken(t *testing.T) {
	ClearTokens()
	typ := "UnitTestStore"
	Register(typ, makeFactory("store", "Authorization", "Bearer 123"))

	h, v, name, err := AcquireAndStoreFromMap(context.Background(), "unitteststore", map[string]interface{}{})
	if err != nil {
		t.Fatalf("AcquireAndStoreFromMap err: %v", err)
	}
	if h == "" || v == "" || name == "" {
		t.Fatalf("expected non-empty results, got h=%q v=%q name=%q", h, v, name)
	}
	gh, gv, ok := GetToken(name)
	if !ok || gh != h || gv != v {
		t.Fatalf("expected token stored: ok=%v gh=%q gv=%q; want h=%q v=%q", ok, gh, gv, h, v)
	}
}

func TestRegistry_UnsupportedType_ReturnsError(t *testing.T) {
	if _, _, _, err := AcquireFromMap(context.Background(), "does-not-exist", nil); err == nil {
		t.Fatalf("expected error for unsupported provider, got nil")
	}
}

func TestRegistry_Register_IgnoresEmptyOrNil(t *testing.T) {
	// empty type
	Register("", nil)
	if _, _, _, err := AcquireFromMap(context.Background(), "", nil); err == nil {
		t.Fatalf("expected error for empty type after Register(\"\", nil)")
	}
}
