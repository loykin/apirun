package env

import (
	"fmt"
	"sync"
)

// VarLazy is a reusable, concurrency-safe lazy resolver for auth/env values tied to an Env.
// It implements fmt.Stringer so it can be safely used in templates and general rendering paths.
// Construct instances via Env helper methods; resolver is provided by Env.
type VarLazy struct {
	once     sync.Once
	res      string
	err      error
	env      *Env
	resolver func(*Env) (string, error)
}

// Value forces acquisition (once) and returns the resolved value and any acquisition error.
func (l *VarLazy) Value() (string, error) {
	// Reuse String() to perform once/do and caching
	_ = l.String()
	return l.res, l.err
}

func (l *VarLazy) String() string {
	l.once.Do(func() {
		if l.resolver == nil {
			l.res = ""
			return
		}
		v, err := l.resolver(l.env)
		if err != nil {
			// keep empty on error; caller paths using RenderGoTemplate just get original or empty
			l.err = err
			l.res = ""
			return
		}
		l.res = v
	})
	// html/template will print the returned string; if empty it prints nothing
	return l.res
}

var _ fmt.Stringer = (*VarLazy)(nil)

// MakeLazy constructs a VarLazy bound to this Env using the provided resolver.
// The resolver decides how to acquire and optionally store the value.
// The logical name may be ignored by the resolver.
func (e *Env) MakeLazy(resolver func(*Env) (string, error)) *VarLazy {
	return &VarLazy{env: e, resolver: resolver}
}
