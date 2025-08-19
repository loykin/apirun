package env

import (
	"bytes"
	"text/template"
)

type Map map[string]string

// Env supports layered variables:
// - Global: variables from config (apply to the whole run)
// - Local: variables from each task (reset per task)
// Lookup and rendering give precedence to Local over Global.
// Note: zero values (nil maps) are handled gracefully.
type Env struct {
	Global Map `yaml:"-" json:"-" mapstructure:"-"`
	Local  Map `yaml:"env" json:"env" mapstructure:"env"`
}

// merged returns a combined map (Global then overridden by Local).
func (e Env) merged() map[string]string {
	m := map[string]string{}
	if e.Global != nil {
		for k, v := range e.Global {
			m[k] = v
		}
	}
	if e.Local != nil {
		for k, v := range e.Local {
			m[k] = v
		}
	}
	return m
}

// Lookup searches Local first, then Global.
func (e Env) Lookup(key string) (string, bool) {
	if e.Local != nil {
		if v, ok := e.Local[key]; ok {
			return v, true
		}
	}
	if e.Global != nil {
		if v, ok := e.Global[key]; ok {
			return v, true
		}
	}
	return "", false
}

// RenderGoTemplate renders strings like {{.username}} with text/template using default Go delimiters.
// The merged map (Local over Global) is used as the dot (.). Missing keys keep the original string unchanged.
func (e Env) RenderGoTemplate(s string) string {
	if len(s) == 0 {
		return s
	}
	t, err := template.New("gotmpl").Option("missingkey=error").Parse(s)
	if err != nil {
		return s
	}
	var buf bytes.Buffer
	if err := t.Execute(&buf, e.merged()); err != nil {
		return s
	}
	return buf.String()
}
