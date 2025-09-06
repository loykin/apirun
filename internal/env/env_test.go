package env

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestEnv_Lookup_BasicAndMissing(t *testing.T) {
	e := Env{Local: map[string]string{"FOO": "bar"}}
	if v, ok := e.Lookup("FOO"); !ok || v != "bar" {
		t.Fatalf("expected FOO=bar, got ok=%v v=%q", ok, v)
	}
	if _, ok := e.Lookup("MISSING"); ok {
		t.Fatalf("expected missing key to return ok=false")
	}
}

func TestEnv_Lookup_PrecedenceAndNil(t *testing.T) {
	// Local overrides Global
	e := Env{Global: map[string]string{"K": "global"}, Local: map[string]string{"K": "local"}}
	if v, ok := e.Lookup("K"); !ok || v != "local" {
		t.Fatalf("expected local to override, got %q (ok=%v)", v, ok)
	}
	// Nil maps should not panic and just return false
	e2 := Env{}
	if _, ok := e2.Lookup("ANY"); ok {
		t.Fatalf("expected ok=false for nil maps")
	}
	// Only Global
	e3 := Env{Global: map[string]string{"G": "v"}}
	if v, ok := e3.Lookup("G"); !ok || v != "v" {
		t.Fatalf("expected global lookup to work, got %q (ok=%v)", v, ok)
	}
}

func TestEnv_RenderGoTemplate_BasicAndMissingKey(t *testing.T) {
	e := Env{Global: map[string]string{"username": "alice", "CITY": "seoul"}}
	in := "user={{.env.username}}, city={{.env.CITY}}"
	out := e.RenderGoTemplate(in)
	expected := "user=alice, city=seoul"
	if out != expected {
		t.Fatalf("expected %q, got %q", expected, out)
	}
	// Missing keys should keep the original string unchanged
	if got := e.RenderGoTemplate("hello {{.missing}}"); got != "hello {{.missing}}" {
		t.Fatalf("expected unchanged when missing key, got %q", got)
	}
}

func TestEnv_RenderGoTemplate_EmptyAndNonTemplateAndParseError(t *testing.T) {
	e := Env{Global: map[string]string{"x": "1"}}
	if got := e.RenderGoTemplate(""); got != "" {
		t.Fatalf("empty input should return empty, got %q", got)
	}
	if got := e.RenderGoTemplate("no templates here"); got != "no templates here" {
		t.Fatalf("non-template string should be unchanged, got %q", got)
	}
	// Parse error => return original string
	bad := "Hello {{.x"
	if got := e.RenderGoTemplate(bad); got != bad {
		t.Fatalf("parse error should return original, got %q", got)
	}
}

func TestEnv_RenderGoTemplate_LocalOverridesGlobal(t *testing.T) {
	e := Env{Global: map[string]string{"name": "global"}, Local: map[string]string{"name": "local"}}
	if got := e.RenderGoTemplate("hi {{.env.name}}"); got != "hi local" {
		t.Fatalf("expected local override, got %q", got)
	}
}

func TestEnv_RenderGoTemplate_GroupedEnvAndAuth(t *testing.T) {
	// .env should expose merged map with Local overriding Global
	e := Env{
		Global: map[string]string{"base": "http://g", "user": "global"},
		Local:  map[string]string{"user": "local"},
		Auth:   map[string]string{"kc": "Bearer TKN"},
	}
	in := "url={{.env.base}}/u={{.env.user}} auth={{.auth.kc}}"
	out := e.RenderGoTemplate(in)
	expected := "url=http://g/u=local auth=Bearer TKN"
	if out != expected {
		t.Fatalf("expected %q, got %q", expected, out)
	}
}

func TestEnv_RenderGoTemplate_HtmlEscaping(t *testing.T) {
	// html/template should escape by default
	e := Env{Global: map[string]string{"danger": "<script>alert('x')</script>"}}
	got := e.RenderGoTemplate("X={{.env.danger}}")
	// Expect escaped characters
	if !strings.Contains(got, "&lt;script&gt;") || !strings.Contains(got, "&lt;/script&gt;") {
		t.Fatalf("expected HTML-escaped output, got %q", got)
	}
}

func TestEnv_UnmarshalYAML_SuccessAndError(t *testing.T) {
	// Success: mapping decoded into Local
	type wrapper struct {
		E Env `yaml:"env"`
	}
	var w wrapper
	data := []byte("env:\n  A: alpha\n  B: beta\n")
	if err := yaml.Unmarshal(data, &w); err != nil {
		t.Fatalf("unmarshal mapping error: %v", err)
	}
	if w.E.Local["A"] != "alpha" || w.E.Local["B"] != "beta" {
		t.Fatalf("unexpected Local: %#v", w.E.Local)
	}

	// Error: non-mapping env should error
	var w2 wrapper
	bad := []byte("env: [1, 2, 3]\n")
	if err := yaml.Unmarshal(bad, &w2); err == nil {
		t.Fatalf("expected error for non-mapping env, got nil")
	}
}
