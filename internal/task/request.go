package task

import (
	"os"
	"path/filepath"
	"strings"

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
// Returns an error if the body template fails to parse/execute.
func (r RequestSpec) Render(env env.Env) (map[string]string, map[string]string, string, error) {
	hdrs := renderHeaders(env, r.Headers)
	queries := renderQueries(env, r.Queries)

	if r.BodyFile == "" && r.Body == "" {
		return hdrs, queries, "", nil
	}

	// Determine body source: BodyFile if provided, otherwise Body
	var body string
	if strings.TrimSpace(r.BodyFile) != "" {
		path := r.BodyFile
		path = env.RenderGoTemplate(path)
		path = filepath.Clean(path)
		if data, err := os.ReadFile(path); err == nil {
			body = string(data)
		} else {
			return hdrs, queries, "", err
		}
	} else {
		body = r.Body
	}

	body, err := renderBody(env, body)
	if err != nil {
		return hdrs, queries, "", err
	}

	return hdrs, queries, body, nil
}
