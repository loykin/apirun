package task

import (
	"strings"

	"github.com/loykin/apimigrate/pkg/auth"
	"github.com/loykin/apimigrate/pkg/env"
)

type RequestSpec struct {
	AuthName string   `yaml:"auth_name" json:"auth_name"`
	Method   string   `yaml:"method" json:"method"`
	URL      string   `yaml:"url" json:"url"`
	Headers  []Header `yaml:"headers" json:"headers"`
	Queries  []Query  `yaml:"queries" json:"queries"`
	Body     string   `yaml:"body" json:"body"`
}

// Render builds headers, query params and body applying Go template rendering using Env.
// It also injects Authorization header from AuthName if present in env and not already set.
func (r RequestSpec) Render(env env.Env) (map[string]string, map[string]string, string) {
	hdrs := make(map[string]string)
	for _, h := range r.Headers {
		if h.Name == "" {
			continue
		}
		val := h.Value
		if strings.Contains(val, "{{") {
			hdrs[h.Name] = env.RenderGoTemplate(val)
		} else {
			hdrs[h.Name] = val
		}
	}
	if r.AuthName != "" {
		// 1) Try centralized token manager (preferred)
		if h, v, ok := auth.GetToken(r.AuthName); ok {
			if _, exists := hdrs[h]; !exists {
				hdrs[h] = v
			}
		} else {
			// 2) Backward-compatible fallback: look up from Env using layered lookup by upper-cased key
			key := strings.ToUpper(r.AuthName)
			if v, ok := env.Lookup(key); ok {
				if _, exists := hdrs["Authorization"]; !exists {
					hdrs["Authorization"] = v
				}
			}
		}
	}

	queries := make(map[string]string)
	for _, q := range r.Queries {
		if q.Name == "" {
			continue
		}
		val := q.Value
		if strings.Contains(val, "{{") {
			queries[q.Name] = env.RenderGoTemplate(val)
		} else {
			queries[q.Name] = val
		}
	}

	body := r.Body
	if strings.Contains(body, "{{") {
		body = env.RenderGoTemplate(body)
	}
	return hdrs, queries, body
}
