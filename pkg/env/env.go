package env

import (
	"bytes"
	"fmt"
	"html/template"
	"strings"
	"sync"

	"gopkg.in/yaml.v3"
)

type Str string

func (s Str) String() string { return string(s) }

func FromStringMap(m map[string]string) Map {
	if m == nil {
		return nil
	}
	out := Map{}
	for k, v := range m {
		out[k] = Str(v)
	}
	return out
}

type Val interface {
	String() string
}

// Map is a generic value map where each value can be a plain string (fmt.Stringer)
// or a lazy/computed value implementing String().
// For convenience, we will store strings using Str type below.
type Map map[string]Val

// New returns a pointer to Env with all internal maps initialized.
// Using this helps avoid nil map checks when populating Auth/Global/Local.
func New() *Env {
	return &Env{Auth: Map{}, Global: Map{}, Local: Map{}}
}

// Seal marks the Env as immutable for Set operations.
func (e *Env) Seal() {
	if e != nil {
		e.mu.Lock()
		e.sealed = true
		e.mu.Unlock()
	}
}

// Unseal re-allows Set operations (testing/initialization only).
func (e *Env) Unseal() {
	if e != nil {
		e.mu.Lock()
		e.sealed = false
		e.mu.Unlock()
	}
}

// Clone performs a deep copy of the Env maps. Lazy values (Stringer) are copied by reference.
func (e *Env) Clone() *Env {
	if e == nil {
		return New()
	}
	e.mu.RLock()
	defer e.mu.RUnlock()
	out := &Env{Auth: Map{}, Global: Map{}, Local: Map{}}
	for k, v := range e.Auth {
		out.Auth[k] = v
	}
	for k, v := range e.Global {
		out.Global[k] = v
	}
	for k, v := range e.Local {
		out.Local[k] = v
	}
	return out
}

// GetString reads a value from the chosen map ("auth","global","local").
func (e *Env) GetString(mapName, key string) string {
	if e == nil {
		return ""
	}
	e.mu.RLock()
	defer e.mu.RUnlock()
	var m Map
	switch normalizeMapName(mapName) {
	case "auth":
		m = e.Auth
	case "local":
		m = e.Local
	default:
		m = e.Global
	}
	if m == nil {
		return ""
	}
	if v, ok := m[key]; ok && v != nil {
		return v.String()
	}
	return ""
}

// SetString sets a string into the chosen map. Returns error if sealed.
func (e *Env) SetString(mapName, key, val string) error {
	if e == nil {
		return nil
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.sealed {
		return fmt.Errorf("env: sealed (immutable)")
	}
	var m *Map
	switch normalizeMapName(mapName) {
	case "auth":
		m = &e.Auth
	case "local":
		m = &e.Local
	default:
		m = &e.Global
	}
	if *m == nil {
		*m = Map{}
	}
	(*m)[key] = Str(val)
	return nil
}

func normalizeMapName(n string) string {
	switch strings.ToLower(strings.TrimSpace(n)) {
	case "auth":
		return "auth"
	case "local":
		return "local"
	default:
		return "global"
	}
}

// Env supports layered variables:
// - Auth: variables from auth providers (apply to the whole run)
// - Global: variables from config (apply to the whole run)
// - Local: variables from each task (reset per task)
// Lookup and rendering give precedence to Local over Global.
// Note: zero values (nil maps) are handled gracefully.
type Env struct {
	mu     sync.RWMutex
	Auth   Map `yaml:"-" json:"-" mapstructure:"-"`
	Global Map `yaml:"-" json:"-" mapstructure:"-"`
	Local  Map `yaml:"-" json:"env" mapstructure:"env"`
	sealed bool
}

// UnmarshalYAML allows decoding a plain mapping under the `env` key directly into Local.
func (e *Env) UnmarshalYAML(value *yaml.Node) error {
	if value == nil {
		return nil
	}
	// Attempt to decode mapping of string->string
	var m map[string]string
	if err := value.Decode(&m); err != nil {
		return err
	}
	e.Local = FromStringMap(m)
	return nil
}

// merged returns a combined map (Global then overridden by Local).
func (e *Env) merged() map[string]string {
	m := map[string]string{}
	if e != nil && e.Global != nil {
		for k, v := range e.Global {
			if v != nil {
				m[k] = v.String()
			}
		}
	}
	if e != nil && e.Local != nil {
		for k, v := range e.Local {
			if v != nil {
				m[k] = v.String()
			}
		}
	}
	return m
}

// dataForTemplate builds the dot object for template execution supporting both
// legacy flat lookups (e.g., {{.kc_base}}) and the new
// grouped lookups ({{.env.kc_base}}, {{.auth.keycloak}}).
func (e *Env) dataForTemplate() map[string]interface{} {
	data := map[string]interface{}{}
	// Build merged env for grouped access only (no flat exposure)
	merged := e.merged()
	// Grouped access under .env only
	data["env"] = merged
	// Grouped access under .auth: expose existing values (string or Stringer)
	am := map[string]interface{}{}
	if e != nil && e.Auth != nil {
		for k, v := range e.Auth {
			am[k] = v
		}
	}
	data["auth"] = am
	return data
}

// Lookup searches Local first, then Global.
func (e *Env) Lookup(key string) (string, bool) {
	if e != nil && e.Local != nil {
		if v, ok := e.Local[key]; ok {
			if v != nil {
				return v.String(), true
			}
		}
	}
	if e != nil && e.Global != nil {
		if v, ok := e.Global[key]; ok {
			if v != nil {
				return v.String(), true
			}
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

// RenderGoTemplateErr behaves like RenderGoTemplate but returns an error when
// the template cannot be parsed or executed (including missing keys due to missingkey=error).
// This is useful for critical contexts like HTTP body rendering where silent fallback would hide issues.
func (e *Env) RenderGoTemplateErr(s string) (string, error) {
	if len(s) == 0 {
		return s, nil
	}
	t, err := template.New("gotmpl").Option("missingkey=error").Parse(s)
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	if err := t.Execute(&buf, e.dataForTemplate()); err != nil {
		return "", err
	}
	return buf.String(), nil
}
