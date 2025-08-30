package oauth2

import (
	"context"
)

// Method is the local interface implemented by oauth2 auth methods.
// It matches the parent auth.Method signature (Acquire only, returning value).
type Method interface {
	Acquire(ctx context.Context) (value string, err error)
}
