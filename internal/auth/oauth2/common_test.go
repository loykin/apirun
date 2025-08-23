package oauth2

import (
	"context"
	"testing"
)

// small helper to ensure Acquire signature compiles with context
func TestMethodAcquireSignature(t *testing.T) {
	// Use a dummy Method via adapter to ensure types line up
	d := Adapter{M: dummyMethod{name: "x"}}
	_, _, _ = d.Acquire(context.Background())
}

type dummyMethod struct{ name string }

func (d dummyMethod) Name() string { return d.name }
func (d dummyMethod) Acquire(_ context.Context) (string, string, error) {
	return "H", "V", nil
}
