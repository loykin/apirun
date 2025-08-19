package task

import (
	"testing"

	env2 "github.com/loykin/apimigrate/pkg/env"
)

func TestEnv_RenderGoTemplate_BasicAndMissingAndEmpty(t *testing.T) {
	// basic render
	env := env2.Env{Local: map[string]string{"USER": "alice", "city": "Seoul"}}
	got := env.RenderGoTemplate("hello {{.USER}} from {{.city}}")
	if got != "hello alice from Seoul" {
		t.Fatalf("unexpected render result: %q", got)
	}

	// missing key leaves template unchanged
	got2 := env.RenderGoTemplate("{{.UNKNOWN}} ok")
	if got2 != "{{.UNKNOWN}} ok" {
		t.Fatalf("expected missing key to keep input, got: %q", got2)
	}

	// empty input returns empty
	if env.RenderGoTemplate("") != "" {
		t.Fatalf("empty input should return empty string")
	}

	// nil env map returns input as-is
	nilEnv := env2.Env{}
	in := "{{.FOO}}"
	if nilEnv.RenderGoTemplate(in) != in {
		t.Fatalf("nil env should keep input unchanged")
	}
}

func TestRequest_Render_TemplatingAndAuthInjection(t *testing.T) {
	env := env2.Env{Local: map[string]string{
		"TOKEN":          "Bearer abc",
		"name":           "bob",
		"CITY":           "busan",
		"forwarded_data": "zzz",
	}}

	req := RequestSpec{
		AuthName: "token", // should lookup upper-cased key
		Headers: []Header{
			{Name: "X-Name", Value: "{{.name}}"},
			{Name: "Forwarded-Data", Value: "{{.forwarded_data}}"},
		},
		Queries: []Query{
			{Name: "city", Value: "{{.CITY}}"},
			{Name: "static", Value: "value"},
		},
		Body: `{"hello": "{{.name}}"}`,
	}

	hdrs, queries, body := req.Render(env)

	// Headers templated
	if hdrs["X-Name"] != "bob" {
		t.Fatalf("header X-Name not rendered, got %q", hdrs["X-Name"])
	}
	if hdrs["Forwarded-Data"] != "zzz" {
		t.Fatalf("header Forwarded-Data not rendered, got %q", hdrs["Forwarded-Data"])
	}
	// Authorization injected
	if hdrs["Authorization"] != "Bearer abc" {
		t.Fatalf("expected Authorization to be injected, got %q", hdrs["Authorization"])
	}

	// Queries templated and passthrough
	if queries["city"] != "busan" {
		t.Fatalf("query city not rendered, got %q", queries["city"])
	}
	if queries["static"] != "value" {
		t.Fatalf("query static passthrough failed, got %q", queries["static"])
	}

	// Body templated
	if body != `{"hello": "bob"}` {
		t.Fatalf("body not rendered, got %q", body)
	}
}

func TestRequest_Render_DoesNotOverrideAuthorization(t *testing.T) {
	env := env2.Env{Local: map[string]string{"KEYCLOAK": "Bearer should-not-use"}}
	req := RequestSpec{
		AuthName: "keycloak",
		Headers:  []Header{{Name: "Authorization", Value: "Bearer preset"}},
	}

	hdrs, _, _ := req.Render(env)
	if hdrs["Authorization"] != "Bearer preset" {
		t.Fatalf("Authorization should not be overridden, got %q", hdrs["Authorization"])
	}
}

func TestRequest_Render_PassThroughNoTemplates(t *testing.T) {
	env := env2.Env{Local: map[string]string{"FOO": "bar"}}
	req := RequestSpec{
		Headers: []Header{{Name: "A", Value: "x"}},
		Queries: []Query{{Name: "q", Value: "y"}},
		Body:    "plain",
	}
	hdrs, queries, body := req.Render(env)
	if hdrs["A"] != "x" || queries["q"] != "y" || body != "plain" {
		t.Fatalf("passthrough failed: hdr=%v queries=%v body=%q", hdrs, queries, body)
	}
}
