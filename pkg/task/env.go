package task

import (
	"bytes"
	"text/template"
)

type EnvMap map[string]string

type Env struct {
	EnvMap EnvMap
}

// RenderGoTemplate renders strings like {{.username}} with text/template using default Go delimiters.
// The Env.EnvMap is used as the dot (.). Missing keys keep the original string unchanged by returning the input.
func (e Env) RenderGoTemplate(s string) string {
	if len(s) == 0 || e.EnvMap == nil {
		return s
	}
	t, err := template.New("gotmpl").Option("missingkey=error").Parse(s)
	if err != nil {
		return s
	}
	var buf bytes.Buffer
	if err := t.Execute(&buf, e.EnvMap); err != nil {
		return s
	}
	return buf.String()
}
