package task

import (
	"testing"

	"github.com/loykin/apimigrate/internal/env"
)

// FuzzAllowedStatus ensures that arbitrary status patterns do not panic and always return a map.
func FuzzAllowedStatus(f *testing.F) {
	f.Add("200", "")
	f.Add("${{.x}}", "201")
	f.Fuzz(func(t *testing.T, pat string, val string) {
		r := ResponseSpec{ResultCode: []string{pat}}
		e := env.Env{Global: map[string]string{"x": val}}
		_ = r.AllowedStatus(e)
	})
}

// FuzzExtractEnv ensures arbitrary JSON bytes and random gjson paths do not panic.
func FuzzExtractEnv(f *testing.F) {
	f.Add([]byte(`{"a":{"b":[{"id":"1"}]}}`), "id", "a.b.0.id")
	f.Add([]byte(`not json`), "x", "a..broken")
	f.Fuzz(func(t *testing.T, body []byte, k, path string) {
		if len(body) > 1<<16 {
			body = body[:1<<16]
		}
		r := ResponseSpec{EnvFrom: map[string]string{k: path}}
		_ = r.ExtractEnv(body)
	})
}
