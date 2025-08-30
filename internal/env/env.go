package env

import (
	"bytes"
	"html/template"

	"gopkg.in/yaml.v3"
)

type Map map[string]string

// New returns a pointer to Env with all internal maps initialized.
// Using this helps avoid nil map checks when populating Auth/Global/Local.
func New() *Env {
	return &Env{Auth: map[string]string{}, Global: map[string]string{}, Local: map[string]string{}}
}

// Env supports layered variables:
// - Auth: variables from auth providers (apply to the whole run)
// - Global: variables from config (apply to the whole run)
// - Local: variables from each task (reset per task)
// Lookup and rendering give precedence to Local over Global.
// Note: zero values (nil maps) are handled gracefully.
type Env struct {
	Auth   Map `yaml:"-" json:"-" mapstructure:"-"`
	Global Map `yaml:"-" json:"-" mapstructure:"-"`
	Local  Map `yaml:"-" json:"env" mapstructure:"env"`
}

// UnmarshalYAML allows decoding a plain mapping under the `env` key directly into Local.
func (e *Env) UnmarshalYAML(value *yaml.Node) error {
	if value == nil {
		return nil
	}
	// Attempt to decode mapping of string->string
	var m map[string]string
	if err := value.Decode(&m); err != nil {
		// If it's not a simple mapping, leave Local as nil and return the error to signal misuse.
		return err
	}
	e.Local = m
	return nil
}

// merged returns a combined map (Global then overridden by Local).
func (e *Env) merged() map[string]string {
	m := map[string]string{}
	if e != nil && e.Global != nil {
		for k, v := range e.Global {
			m[k] = v
		}
	}
	if e != nil && e.Local != nil {
		for k, v := range e.Local {
			m[k] = v
		}
	}
	return m
}

// dataForTemplate builds the dot object for template execution supporting both
// legacy flat lookups (e.g., {{.kc_base}}) and the new
// grouped lookups ({{.env.kc_base}}, {{.auth.keycloak}}).
func (e *Env) dataForTemplate() map[string]interface{} {
	data := map[string]interface{}{}
	// Expose merged env flat for backward compatibility
	merged := e.merged()
	for k, v := range merged {
		data[k] = v
	}
	// Grouped access under .env
	data["env"] = merged
	// Grouped access under .auth (may be nil)
	if e != nil && e.Auth != nil {
		// Copy to interface map to avoid accidental mutation
		a := map[string]string{}
		for k, v := range e.Auth {
			a[k] = v
		}
		data["auth"] = a
	} else {
		data["auth"] = map[string]string{}
	}
	return data
}

// Lookup searches Local first, then Global.
func (e *Env) Lookup(key string) (string, bool) {
	if e != nil && e.Local != nil {
		if v, ok := e.Local[key]; ok {
			return v, true
		}
	}
	if e != nil && e.Global != nil {
		if v, ok := e.Global[key]; ok {
			return v, true
		}
	}
	return "", false
}

// RenderGoTemplate renders strings like {{.username}} with html/template using default Go delimiters.
// Missing keys keep the original string unchanged.
// Note: html/template escapes HTML by default to mitigate XSS when used in HTML contexts.
func (e *Env) RenderGoTemplate(s string) string {
	if len(s) == 0 {
		return s
	}
	t, err := template.New("gotmpl").Option("missingkey=error").Parse(s)
	if err != nil {
		return s
	}
	var buf bytes.Buffer
	if err := t.Execute(&buf, e.dataForTemplate()); err != nil {
		return s
	}
	return buf.String()
}
