package task

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/go-resty/resty/v2"
)

type Up struct {
	Name     string       `yaml:"name" json:"name"`
	Env      Env          `yaml:"env" json:"env"`
	Request  RequestSpec  `yaml:"request" json:"request"`
	Response ResponseSpec `yaml:"response" json:"response"`
}

// UpSpec is kept as an alias for backward compatibility with existing callers/tests.
// Methods defined on Up are available on UpSpec as well.
type UpSpec = Up

// Execute runs this Up specification against the provided HTTP method and URL.
// It performs templating, sends the request, validates the response and extracts env.
func (u Up) Execute(ctx context.Context, method, url string) (*ExecResult, error) {
	// Build request components via RequestSpec method
	hdrs, queries, body := u.Request.Render(u.Env)

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
	switch strings.ToUpper(method) {
	case http.MethodGet:
		resp, err = req.Get(url)
	case http.MethodPost:
		resp, err = req.Post(url)
	case http.MethodPut:
		resp, err = req.Put(url)
	case http.MethodPatch:
		resp, err = req.Patch(url)
	case http.MethodDelete:
		resp, err = req.Delete(url)
	default:
		return nil, fmt.Errorf("unsupported method: %s", method)
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
