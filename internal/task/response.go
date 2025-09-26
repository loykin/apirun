package task

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/loykin/apirun/pkg/env"
	"github.com/tidwall/gjson"
)

type ResponseSpec struct {
	// ResultCode entries may be integers or go-template strings (e.g., {{.result_code}}) in YAML.
	// We load them as strings to allow templating at execution time.
	ResultCode []string          `yaml:"result_code"`
	EnvFrom    map[string]string `yaml:"env_from"`
	// EnvMissing controls behavior when a configured EnvFrom mapping cannot be extracted from response body.
	// Allowed values: "skip" (default) – ignore missing variables; "fail" – treat as error.
	EnvMissing string `yaml:"env_missing"`
}

// AllowedStatus renders ResultCode against provided env vars and returns a set of allowed codes.
func (r ResponseSpec) AllowedStatus(env *env.Env) map[int]struct{} {
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
func (r ResponseSpec) ValidateStatus(status int, env *env.Env) error {
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
// It respects EnvMissing policy: "skip" (default) ignores missing variables; "fail" returns an error.
func (r ResponseSpec) ExtractEnv(body []byte) (map[string]string, error) {
	// Ensure deterministic behavior regardless of Go's random map iteration order.
	extracted := map[string]string{}
	if len(r.EnvFrom) == 0 || len(body) == 0 {
		return extracted, nil
	}

	policy := strings.ToLower(strings.TrimSpace(r.EnvMissing))
	if policy == "" {
		policy = "skip"
	}

	parsed := gjson.ParseBytes(body)

	// First pass: extract all keys that exist.
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

	// Second pass: if policy is fail, check for any missing keys and return error
	// while preserving the already extracted values from the first pass.
	if policy == "fail" {
		for key, path := range r.EnvFrom {
			p := strings.TrimSpace(path)
			if p == "" {
				continue
			}
			res := parsed.Get(p)
			if !res.Exists() {
				return extracted, fmt.Errorf("missing env_from for key '%s' at path '%s'", key, p)
			}
		}
	}

	return extracted, nil
}
