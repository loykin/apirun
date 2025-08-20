package task

import (
	"fmt"
	"strconv"
	"strings"

	env "github.com/loykin/apimigrate/internal/env"
	"github.com/tidwall/gjson"
)

type ResponseSpec struct {
	// ResultCode entries may be integers or go-template strings (e.g., {{.result_code}}) in YAML.
	// We load them as strings to allow templating at execution time.
	ResultCode []string          `yaml:"result_code" json:"result_code"`
	EnvFrom    map[string]string `yaml:"env_from" json:"env_from"`
}

// AllowedStatus renders ResultCode against provided env vars and returns a set of allowed codes.
func (r ResponseSpec) AllowedStatus(env env.Env) map[int]struct{} {
	allowed := map[int]struct{}{}
	for _, c := range r.ResultCode {
		// Support both {{.var}} and ${{.var}} forms by normalizing the latter
		tpl := c
		if strings.Contains(tpl, "${{") {
			tpl = strings.ReplaceAll(tpl, "${{", "{{")
		}
		rendered := env.RenderGoTemplate(tpl)
		rendered = strings.TrimSpace(rendered)
		if rendered == "" {
			continue
		}
		if n, err := strconv.Atoi(rendered); err == nil {
			allowed[n] = struct{}{}
		}
	}
	return allowed
}

// ValidateStatus checks whether status code is allowed. If no ResultCode specified, all statuses are considered success.
func (r ResponseSpec) ValidateStatus(status int, env env.Env) error {
	allowed := r.AllowedStatus(env)
	if len(allowed) == 0 {
		// No restrictions => any status is success
		return nil
	}
	if _, ok := allowed[status]; !ok {
		return fmt.Errorf("status %d not in allowed set", status)
	}
	return nil
}

// ExtractEnv extracts variables from a JSON response body using EnvFrom mappings.
// Paths are evaluated with tidwall/gjson. For backward compatibility, common
// JSONPath forms like "$[0].abc" or "$.a.b" are converted to gjson syntax.
func (r ResponseSpec) ExtractEnv(body []byte) map[string]string {
	extracted := map[string]string{}
	if len(r.EnvFrom) == 0 || len(body) == 0 {
		return extracted
	}

	parsed := gjson.ParseBytes(body)
	for key, path := range r.EnvFrom {
		if path == "" {
			continue
		}
		gp := toGJSONPath(path)
		res := parsed.Get(gp)
		if res.Exists() {
			extracted[key] = anyToString(res.Value())
		}
	}
	return extracted
}

// toGJSONPath converts a subset of common JSONPath-like expressions to gjson syntax.
// Supported conversions:
// - Remove leading "$" or "$."
// - Convert bracket numeric selectors like [0] to .0
// - Convert bracket quoted keys like ['name'] to .name
// It is best-effort and only intended to cover simple use cases in this project.
func toGJSONPath(p string) string {
	if p == "" {
		return p
	}
	s := strings.TrimSpace(p)
	if strings.HasPrefix(s, "$.") {
		s = s[2:]
	} else if strings.HasPrefix(s, "$") {
		s = s[1:]
	}
	// Replace bracket segments [0] or ['key'] iteratively
	for {
		start := strings.Index(s, "[")
		if start == -1 {
			break
		}
		endRel := strings.Index(s[start:], "]")
		if endRel == -1 {
			break
		}
		end := start + endRel
		inner := s[start+1 : end]
		repl := ""
		if isDigits(inner) {
			repl = "." + inner
		} else if len(inner) >= 2 && inner[0] == '\'' && inner[len(inner)-1] == '\'' {
			key := inner[1 : len(inner)-1]
			repl = "." + key
		} else {
			// Unknown pattern; stop converting to avoid mangling
			break
		}
		s = s[:start] + repl + s[end+1:]
	}
	// Remove leading dot if present
	s = strings.TrimPrefix(s, ".")
	return s
}

func isDigits(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		ch := s[i]
		if ch < '0' || ch > '9' {
			return false
		}
	}
	return true
}
