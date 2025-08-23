package auth

import (
	"sync"
	"testing"
)

func TestToken_SetGet_CaseInsensitive(t *testing.T) {
	ClearTokens()
	SetToken("KeyCloak", "Authorization", "Bearer XYZ")

	if h, v, ok := GetToken("keycloak"); !ok || h != "Authorization" || v != "Bearer XYZ" {
		t.Fatalf("GetToken case-insensitive failed: ok=%v h=%q v=%q", ok, h, v)
	}

	if h, v, ok := GetToken("KEYCLOAK"); !ok || h != "Authorization" || v != "Bearer XYZ" {
		t.Fatalf("GetToken case-insensitive (upper) failed: ok=%v h=%q v=%q", ok, h, v)
	}
}

func TestToken_EmptyInputsIgnored(t *testing.T) {
	ClearTokens()

	SetToken("", "Authorization", "v")
	if _, _, ok := GetToken(""); ok {
		t.Fatalf("expected empty name not to be stored")
	}
	SetToken("n1", "", "v")
	if _, _, ok := GetToken("n1"); ok {
		t.Fatalf("expected empty header not to be stored")
	}
	SetToken("n2", "Authorization", "")
	if _, _, ok := GetToken("n2"); ok {
		t.Fatalf("expected empty value not to be stored")
	}

	// A valid one should be retrievable
	SetToken("valid", "X-Auth", "ok")
	if h, v, ok := GetToken("valid"); !ok || h != "X-Auth" || v != "ok" {
		t.Fatalf("valid token missing: ok=%v h=%q v=%q", ok, h, v)
	}
}

func TestToken_ClearTokens(t *testing.T) {
	ClearTokens()
	SetToken("a", "H", "V")
	ClearTokens()
	if _, _, ok := GetToken("a"); ok {
		t.Fatalf("expected tokens cleared")
	}
}

// A light concurrency sanity check to ensure no panics/data races in basic usage
func TestToken_ConcurrentAccess(t *testing.T) {
	ClearTokens()
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			SetToken("name", "H", "V")
			_, _, _ = GetToken("NAME")
		}(i)
	}
	wg.Wait()
}
