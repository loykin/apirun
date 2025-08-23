package task

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/loykin/apimigrate/internal/env"
	"github.com/tidwall/gjson"
)

type ResponseSpec struct {
	// ResultCode entries may be integers or go-template strings (e.g., {{.result_code}}) in YAML.
	// We load them as strings to allow templating at execution time.
	ResultCode []string          `yaml:"result_code"`
	EnvFrom    map[string]string `yaml:"env_from"`
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
// Paths are evaluated with tidwall/gjson and are expected to be valid gjson paths.
func (r ResponseSpec) ExtractEnv(body []byte) map[string]string {
	extracted := map[string]string{}
	if len(r.EnvFrom) == 0 || len(body) == 0 {
		return extracted
	}

	parsed := gjson.ParseBytes(body)
	for key, path := range r.EnvFrom {
		p := strings.TrimSpace(path)
		if p == "" {
			continue
		}
		res := parsed.Get(p)
		if res.Exists() {
			extracted[key] = anyToString(res.Value())
		}
	}
	return extracted
}
