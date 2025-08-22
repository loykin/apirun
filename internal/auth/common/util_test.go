package common

import "testing"

func TestHeaderOrDefault_Common(t *testing.T) {
	if got := HeaderOrDefault(""); got != "Authorization" {
		t.Fatalf("expected Authorization, got %q", got)
	}
	if got := HeaderOrDefault("  X-API-Key "); got != "X-API-Key" {
		t.Fatalf("expected X-API-Key, got %q", got)
	}
}
