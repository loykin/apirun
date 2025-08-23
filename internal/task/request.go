package task

import (
	"os"
	"strings"

	"github.com/loykin/apimigrate/internal/auth"
	"github.com/loykin/apimigrate/internal/env"
)

type RequestSpec struct {
	AuthName string   `yaml:"auth_name"`
	Method   string   `yaml:"method"`
	URL      string   `yaml:"url"`
	Headers  []Header `yaml:"headers"`
	Queries  []Query  `yaml:"queries"`
	Body     string   `yaml:"body"`
	BodyFile string   `yaml:"body_file"`
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
		hdrs[h.Name] = env.RenderGoTemplate(val)
	}
	if r.AuthName != "" {
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
		queries[q.Name] = env.RenderGoTemplate(val)
	}

	// Determine body source: BodyFile if provided, otherwise Body
	var body string
	if strings.TrimSpace(r.BodyFile) != "" {
		path := r.BodyFile
		path = env.RenderGoTemplate(path)
		if data, err := os.ReadFile(path); err == nil {
			body = string(data)
		} else {
			// If file read fails, keep body empty to let caller handle error statuses
			body = ""
		}
	} else {
		body = r.Body
	}

	body = env.RenderGoTemplate(body)
	return hdrs, queries, body
}
