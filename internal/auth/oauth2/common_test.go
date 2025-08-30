package oauth2

import (
	"context"
	"testing"
)

// small helper to ensure Acquire signature compiles with context
func TestMethodAcquireSignature(t *testing.T) {
	// Use a dummy Method via adapter to ensure types line up
	d := Adapter{M: dummyMethod{}}
	_, _ = d.Acquire(context.Background())
}

type dummyMethod struct{}

func (d dummyMethod) Acquire(_ context.Context) (string, error) {
	return "V", nil
}
