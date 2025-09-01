package task

import (
	"testing"

	ienv "github.com/loykin/apimigrate/internal/env"
)

// FuzzTask is a single fuzz entry point for the internal/task package.
// It exercises RequestSpec.Render, ResponseSpec.AllowedStatus, and
// ResponseSpec.ExtractEnv to prevent ambiguity when running `go test -fuzz`.
func FuzzTask(f *testing.F) {
	// Seed inputs for request rendering
	f.Add("auth", "X-Name", "{{.who}}", "q", "{{.qv}}", "Hello, {{.who}}!",
		"200", "", string([]byte("{\"a\":1}")), "id", "a")
	f.Add("", "Authorization", "Bearer {{.auth.k}}", "x", "1", "{\"a\":1}",
		"${{.x}}", "201", string([]byte(`{"a":{"b":[{"id":"1"}]}}`)), "rid", "a.b.0.id")

	f.Fuzz(func(t *testing.T,
		authName, hName, hVal, qName, qVal, body string,
		pat, val, bodyJSON, envKey, gjsonPath string,
	) {
		// 1) RequestSpec.Render
		r := RequestSpec{
			AuthName: authName,
			Headers:  []Header{{Name: hName, Value: hVal}},
			Queries:  []Query{{Name: qName, Value: qVal}},
			Body:     body,
		}
		e := ienv.Env{Global: ienv.Map{"who": "world", "qv": "ok", "x": val}, Local: ienv.Map{"who": "bob"}, Auth: ienv.Map{"k": "tok"}}
		_, _, _ = r.Render(e)

		// 2) ResponseSpec.AllowedStatus
		r2 := ResponseSpec{ResultCode: []string{pat}}
		_ = r2.AllowedStatus(ienv.Env{Global: ienv.Map{"x": val}})

		// 3) ResponseSpec.ExtractEnv
		if len(bodyJSON) > 1<<16 {
			bodyJSON = bodyJSON[:1<<16]
		}
		r3 := ResponseSpec{EnvFrom: map[string]string{envKey: gjsonPath}}
		_, _ = r3.ExtractEnv([]byte(bodyJSON))
	})
}
