package util

import (
	"reflect"
	"testing"

	"github.com/loykin/apimigrate/pkg/env"
)

func TestRenderAnyTemplate_BasicGoTemplateOnly(t *testing.T) {
	e := env.Env{Global: env.FromStringMap(map[string]string{
		"name":   "Alice",
		"nested": "World",
	})}

	in := map[string]interface{}{
		"plain":            "no templating here",
		"go_template":      "Hello, {{.env.name}}!",
		"alt_dollar_brace": "${.name}",
		"alt_dollar_curly": "${{.env.name}}",
		"slice": []interface{}{
			"Item: {{.env.nested}}",
			"${.nested}",
		},
	}

	outAny := RenderAnyTemplate(in, &e)
	out, ok := outAny.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map[string]interface{} output, got %T", outAny)
	}

	if got := out["plain"].(string); got != "no templating here" {
		t.Fatalf("plain mismatch: %q", got)
	}
	if got := out["go_template"].(string); got != "Hello, Alice!" {
		t.Fatalf("go_template rendered wrong: %q", got)
	}
	if got := out["alt_dollar_brace"].(string); got != "${.name}" {
		t.Fatalf("alt_dollar_brace should remain unchanged, got: %q", got)
	}
	if got := out["alt_dollar_curly"].(string); got != "$Alice" {
		t.Fatalf("alt_dollar_curly expected to render inner Go template with leading $, got: %q", got)
	}

	expSlice := []interface{}{"Item: World", "${.nested}"}
	if got := out["slice"]; !reflect.DeepEqual(got, expSlice) {
		t.Fatalf("slice mismatch: %#v", got)
	}
}
