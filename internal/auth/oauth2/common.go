package oauth2

import (
	"context"
)

// Method is the local interface implemented by oauth2 auth methods.
// It matches the parent auth.Method signature (Name, Acquire).
type Method interface {
	Name() string
	Acquire(ctx context.Context) (header string, value string, err error)
}
