package task

import (
	"context"
	"fmt"
	"strings"

	"github.com/loykin/apimigrate/internal/env"
)

type Up struct {
	Name     string       `yaml:"name"`
	Env      env.Env      `yaml:"env"`
	Request  RequestSpec  `yaml:"request"`
	Response ResponseSpec `yaml:"response"`
}

// Execute runs this Up specification against the provided HTTP method and URL.
// It performs templating, sends the request, validates the response and extracts env.
// If RequestSpec.Method or RequestSpec.URL are provided, they override the method/url
// passed as parameters. This allows a single migration directory to hit different endpoints.
func (u Up) Execute(ctx context.Context, method, url string) (*ExecResult, error) {
	// Build request components via RequestSpec method
	hdrs, queries, body, rerr := u.Request.Render(u.Env)
	if rerr != nil {
		return nil, fmt.Errorf("up request body template error: %v", rerr)
	}

	// Determine method and url to use (allow per-request overrides)
	methodToUse := method
	if strings.TrimSpace(u.Request.Method) != "" {
		methodToUse = u.Request.Method
	}
	urlToUse := url
	if strings.TrimSpace(u.Request.URL) != "" {
		urlToUse = u.Request.URL
	}
	// Render URL (RenderGoTemplate is idempotent for non-templates)
	urlToUse = u.Env.RenderGoTemplate(urlToUse)

	req := buildRequest(ctx, hdrs, queries, body)
	resp, err := execByMethod(req, methodToUse, urlToUse)
	if err != nil {
		return nil, err
	}

	status := resp.StatusCode()
	bodyBytes := resp.Body()
	// Validate status via ResponseSpec method
	if err := u.Response.ValidateStatus(status, u.Env); err != nil {
		return &ExecResult{StatusCode: status, ExtractedEnv: map[string]string{}, ResponseBody: string(bodyBytes)}, err
	}

	// Extract env from response body via ResponseSpec method (may error if env_missing=fail)
	extracted, eerr := u.Response.ExtractEnv(bodyBytes)
	if eerr != nil {
		return &ExecResult{StatusCode: status, ExtractedEnv: extracted, ResponseBody: string(bodyBytes)}, eerr
	}
	return &ExecResult{StatusCode: status, ExtractedEnv: extracted, ResponseBody: string(bodyBytes)}, nil
}
