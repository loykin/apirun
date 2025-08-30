package basic

import (
	"context"
	"testing"
)

func TestBasicAdapter_Acquire(t *testing.T) {
	ad := Adapter{C: Config{Username: "a", Password: "b"}}
	v, err := ad.Acquire(context.Background())
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if v == "" {
		t.Fatal("expected non-empty token value")
	}
}
