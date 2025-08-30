package task

import (
	"testing"

	ienv "github.com/loykin/apimigrate/internal/env"
)

// FuzzRequestRender fuzzes RequestSpec.Render focusing on headers/queries/body templating
// (BodyFile is intentionally left empty to avoid filesystem I/O during fuzzing).
func FuzzRequestRender(f *testing.F) {
	f.Add("auth", "X-Name", "{{.who}}", "q", "{{.qv}}", "Hello, {{.who}}!")
	f.Add("", "Authorization", "Bearer {{.auth.k}}", "x", "1", "{\"a\":1}")

	f.Fuzz(func(t *testing.T, authName, hName, hVal, qName, qVal, body string) {
		// Build a request with a single header and query to keep it fast
		r := RequestSpec{
			AuthName: authName,
			Headers:  []Header{{Name: hName, Value: hVal}},
			Queries:  []Query{{Name: qName, Value: qVal}},
			Body:     body,
			// BodyFile left empty by design
		}
		e := ienv.Env{Global: ienv.Map{"who": "world", "qv": "ok"}, Local: ienv.Map{"who": "bob"}, Auth: ienv.Map{"k": "tok"}}
		_, _, _ = r.Render(e)
	})
}
