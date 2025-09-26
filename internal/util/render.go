package util

import (
	"github.com/loykin/apirun/pkg/env"
)

// RenderAnyTemplate walks arbitrary structures (map[string]any, []any) and renders
// all string values using the provided env with standard Go template syntax ({{...}}).
// No alternative syntaxes are supported or normalized here.
// The function returns a new rendered structure (for maps/slices) or the
// original value for non-string scalars.
func RenderAnyTemplate(in interface{}, e *env.Env) interface{} {
	var fn func(v interface{}) interface{}
	fn = func(v interface{}) interface{} {
		switch t := v.(type) {
		case map[string]interface{}:
			m := make(map[string]interface{}, len(t))
			for k, vv := range t {
				m[k] = fn(vv)
			}
			return m
		case []interface{}:
			arr := make([]interface{}, len(t))
			for i := range t {
				arr[i] = fn(t[i])
			}
			return arr
		case string:
			if e == nil {
				return t
			}
			return e.RenderGoTemplate(t)
		default:
			return v
		}
	}
	return fn(in)
}
