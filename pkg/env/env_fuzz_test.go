package env

import "testing"

// FuzzRenderGoTemplate fuzzes the template renderer to ensure it never panics
// on arbitrary inputs and always returns a string (may be identical to input).
func FuzzRenderGoTemplate(f *testing.F) {
	// Seed with a few common patterns
	f.Add("")
	f.Add("plain text")
	f.Add("hello {{.env.name}}")
	f.Add("{{.MISSING}") // malformed template
	f.Add("{{.a}}{{.b}}{{.c}}")

	e := &Env{
		Global: FromStringMap(map[string]string{"name": "world", "a": "1", "b": "2"}),
		Local:  FromStringMap(map[string]string{"c": "3"})}
	f.Fuzz(func(t *testing.T, s string) {
		_ = e.RenderGoTemplate(s)
	})
}
