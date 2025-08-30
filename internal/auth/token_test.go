package auth

import (
	"sync"
	"testing"
)

func TestToken_SetGet_CaseInsensitive(t *testing.T) {
	ClearTokens()
	SetToken("KeyCloak", "Bearer XYZ")

	if v, ok := GetToken("keycloak"); !ok || v != "Bearer XYZ" {
		t.Fatalf("GetToken case-insensitive failed: ok=%v v=%q", ok, v)
	}

	if v, ok := GetToken("KEYCLOAK"); !ok || v != "Bearer XYZ" {
		t.Fatalf("GetToken case-insensitive (upper) failed: ok=%v v=%q", ok, v)
	}
}

func TestToken_EmptyInputsIgnored(t *testing.T) {
	ClearTokens()

	SetToken("", "v")
	if _, ok := GetToken(""); ok {
		t.Fatalf("expected empty name not to be stored")
	}
	SetToken("n1", "")
	if _, ok := GetToken("n1"); ok {
		t.Fatalf("expected empty value not to be stored")
	}

	// A valid one should be retrievable
	SetToken("valid", "ok")
	if v, ok := GetToken("valid"); !ok || v != "ok" {
		t.Fatalf("valid token missing: ok=%v v=%q", ok, v)
	}
}

func TestToken_ClearTokens(t *testing.T) {
	ClearTokens()
	SetToken("a", "V")
	ClearTokens()
	if _, ok := GetToken("a"); ok {
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
			SetToken("name", "V")
			_, _ = GetToken("NAME")
		}(i)
	}
	wg.Wait()
}
