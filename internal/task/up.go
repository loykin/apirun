package task

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/go-resty/resty/v2"
	env "github.com/loykin/apimigrate/internal/env"
)

type Up struct {
	Name     string       `yaml:"name" json:"name"`
	Env      env.Env      `yaml:"env" json:"env"`
	Request  RequestSpec  `yaml:"request" json:"request"`
	Response ResponseSpec `yaml:"response" json:"response"`
}

// UpSpec is kept as an alias for backward compatibility with existing callers/tests.
// Methods defined on Up are available on UpSpec as well.
type UpSpec = Up

// Execute runs this Up specification against the provided HTTP method and URL.
// It performs templating, sends the request, validates the response and extracts env.
// If RequestSpec.Method or RequestSpec.URL are provided, they override the method/url
// passed as parameters. This allows a single migration directory to hit different endpoints.
func (u Up) Execute(ctx context.Context, method, url string) (*ExecResult, error) {
	// Build request components via RequestSpec method
	hdrs, queries, body := u.Request.Render(u.Env)

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

	client := resty.New()
	req := client.R().SetContext(ctx).SetHeaders(hdrs).SetQueryParams(queries)
	if strings.TrimSpace(body) != "" {
		if isJSON(body) {
			req.SetHeader("Content-Type", "application/json")
			req.SetBody([]byte(body))
		} else {
			req.SetBody(body)
		}
	}

	var resp *resty.Response
	var err error
	switch strings.ToUpper(methodToUse) {
	case http.MethodGet:
		resp, err = req.Get(urlToUse)
	case http.MethodPost:
		resp, err = req.Post(urlToUse)
	case http.MethodPut:
		resp, err = req.Put(urlToUse)
	case http.MethodPatch:
		resp, err = req.Patch(urlToUse)
	case http.MethodDelete:
		resp, err = req.Delete(urlToUse)
	default:
		return nil, fmt.Errorf("unsupported method: %s", methodToUse)
	}
	if err != nil {
		return nil, err
	}

	status := resp.StatusCode()
	// Validate status via ResponseSpec method
	if err := u.Response.ValidateStatus(status, u.Env); err != nil {
		return &ExecResult{StatusCode: status, ExtractedEnv: map[string]string{}}, err
	}

	// Extract env from response body via ResponseSpec method
	extracted := u.Response.ExtractEnv(resp.Body())
	return &ExecResult{StatusCode: status, ExtractedEnv: extracted}, nil
}
