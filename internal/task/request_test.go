package task

import (
	"os"
	"path/filepath"
	"testing"

	env2 "github.com/loykin/apimigrate/pkg/env"
)

func boolPtr(b bool) *bool { return &b }

func TestEnv_RenderGoTemplate_BasicAndMissingAndEmpty(t *testing.T) {
	// basic render
	env := env2.Env{Local: env2.FromStringMap(map[string]string{"USER": "alice", "city": "Seoul"})}
	got := env.RenderGoTemplate("hello {{.env.USER}} from {{.env.city}}")
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
	in := "{{.env.FOO}}"
	if nilEnv.RenderGoTemplate(in) != in {
		t.Fatalf("nil env should keep input unchanged")
	}
}

func TestRequest_Render_TemplatingAndAuthInjection(t *testing.T) {
	env := env2.Env{Auth: env2.Map{"kc": env2.Str("Bearer abc")}, Local: env2.FromStringMap(map[string]string{
		"name":           "bob",
		"CITY":           "busan",
		"forwarded_data": "zzz",
	})}

	req := RequestSpec{
		Headers: []Header{
			{Name: "X-Name", Value: "{{.env.name}}"},
			{Name: "Forwarded-Data", Value: "{{.env.forwarded_data}}"},
			{Name: "Authorization", Value: "{{.auth.kc}}"},
		},
		Queries: []Query{
			{Name: "city", Value: "{{.env.CITY}}"},
			{Name: "static", Value: "value"},
		},
		Body: `{"hello": "{{.env.name}}"}`,
	}

	hdrs, queries, body, err := req.Render(&env)
	if err != nil {
		t.Fatalf("unexpected render error: %v", err)
	}

	// Headers templated
	if hdrs["X-Name"] != "bob" {
		t.Fatalf("header X-Name not rendered, got %q", hdrs["X-Name"])
	}
	if hdrs["Forwarded-Data"] != "zzz" {
		t.Fatalf("header Forwarded-Data not rendered, got %q", hdrs["Forwarded-Data"])
	}
	// Authorization templated from .auth.kc
	if hdrs["Authorization"] != "Bearer abc" {
		t.Fatalf("expected Authorization to be templated, got %q", hdrs["Authorization"])
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
	env := env2.Env{Local: env2.FromStringMap(map[string]string{"KEY": "Bearer should-not-use"})}
	req := RequestSpec{
		Headers: []Header{{Name: "Authorization", Value: "Bearer preset"}},
	}

	hdrs, _, _, err := req.Render(&env)
	if err != nil {
		t.Fatalf("unexpected render error: %v", err)
	}
	if hdrs["Authorization"] != "Bearer preset" {
		t.Fatalf("Authorization should not be overridden, got %q", hdrs["Authorization"])
	}
}

func TestRequest_Render_PassThroughNoTemplates(t *testing.T) {
	env := env2.Env{Local: env2.FromStringMap(map[string]string{"FOO": "bar"})}
	req := RequestSpec{
		Headers: []Header{{Name: "A", Value: "x"}},
		Queries: []Query{{Name: "q", Value: "y"}},
		Body:    "plain",
	}
	hdrs, queries, body, err := req.Render(&env)
	if err != nil {
		t.Fatalf("unexpected render error: %v", err)
	}
	if hdrs["A"] != "x" || queries["q"] != "y" || body != "plain" {
		t.Fatalf("passthrough failed: hdr=%v queries=%v body=%q", hdrs, queries, body)
	}
}

func TestRequest_Render_BodyFile(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "apimigrate_body_*.json")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer func() { _ = os.Remove(tmpFile.Name()) }()
	content := `{"a": "{{.env.X}}"}`
	if _, err := tmpFile.WriteString(content); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}
	_ = tmpFile.Close()

	env := env2.Env{Local: env2.FromStringMap(map[string]string{"X": "y"})}
	req := RequestSpec{BodyFile: tmpFile.Name()}
	_, _, body, err := req.Render(&env)
	if err != nil {
		t.Fatalf("unexpected render error: %v", err)
	}
	if body != `{"a": "y"}` {
		t.Fatalf("expected body rendered from file, got %q", body)
	}
}

func TestRequest_Render_BodyFilePathTemplate(t *testing.T) {
	dir := t.TempDir()
	// Create data file with templated content
	data := []byte(`{"v":"{{.env.V}}"}`)
	name := "body.json"
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, data, 0o600); err != nil {
		t.Fatalf("write data file: %v", err)
	}
	// Build a templated path that resolves to the file above
	req := RequestSpec{BodyFile: filepath.Join(dir, "{{.env.N}}")}
	env := env2.Env{Local: env2.FromStringMap(map[string]string{"N": name, "V": "ok"})}
	_, _, body, err := req.Render(&env)
	if err != nil {
		t.Fatalf("render error: %v", err)
	}
	if body != `{"v":"ok"}` {
		t.Fatalf("unexpected body: %q", body)
	}
}

func TestRequest_Render_BodyFileWhitespaceFallbackToBody(t *testing.T) {
	env := env2.Env{Local: env2.FromStringMap(map[string]string{"Y": "z"})}
	req := RequestSpec{BodyFile: "   \t\n  ", Body: "x{{.env.Y}}"}
	_, _, body, err := req.Render(&env)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if body != "xz" {
		t.Fatalf("expected body fallback with templating, got %q", body)
	}
}

func TestRequest_Render_EmptyBoth_NoError(t *testing.T) {
	env := env2.Env{}
	req := RequestSpec{}
	_, _, body, err := req.Render(&env)
	if err != nil {
		t.Fatalf("unexpected error for empty body fields: %v", err)
	}
	if body != "" {
		t.Fatalf("expected empty body, got %q", body)
	}
}

func TestRequest_Render_BodyFileNotFound_Error(t *testing.T) {
	env := env2.Env{Local: env2.FromStringMap(map[string]string{"X": "missing.json"})}
	req := RequestSpec{BodyFile: filepath.Join(t.TempDir(), "{{.env.X}}")}
	_, _, _, err := req.Render(&env)
	if err == nil {
		t.Fatalf("expected error for missing BodyFile, got nil")
	}
}

func TestRequest_Render_RenderBodyFalse_LiteralBody(t *testing.T) {
	env := env2.Env{Local: env2.FromStringMap(map[string]string{"X": "y"})}
	req := RequestSpec{Body: `{"a":"{{.env.X}}","raw":"{{ not_replaced }}"}`, RenderBody: boolPtr(false)}
	_, _, body, err := req.Render(&env)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if body != `{"a":"{{.env.X}}","raw":"{{ not_replaced }}"}` {
		t.Fatalf("expected literal body unchanged, got %q", body)
	}
}

func TestRequest_Render_RenderBodyTrue_Renders(t *testing.T) {
	env := env2.Env{Local: env2.FromStringMap(map[string]string{"X": "y"})}
	req := RequestSpec{Body: `{"a":"{{.env.X}}"}`, RenderBody: boolPtr(true)}
	_, _, body, err := req.Render(&env)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if body != `{"a":"y"}` {
		t.Fatalf("expected rendered body, got %q", body)
	}
}

func TestRequest_Render_BodyFile_RenderBodyFalse(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "apimigrate_body_literal_*.json")
	if err != nil {
		t.Fatalf("tmp: %v", err)
	}
	defer func() { _ = os.Remove(tmpFile.Name()) }()
	content := `{"a": "{{.env.X}}", "raw": "{{ abc }}"}`
	if _, err := tmpFile.WriteString(content); err != nil {
		t.Fatalf("write: %v", err)
	}
	_ = tmpFile.Close()
	env := env2.Env{Local: env2.FromStringMap(map[string]string{"X": "y"})}
	req := RequestSpec{BodyFile: tmpFile.Name(), RenderBody: boolPtr(false)}
	_, _, body, err := req.Render(&env)
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if body != content {
		t.Fatalf("expected content unchanged from file, got %q", body)
	}
}
