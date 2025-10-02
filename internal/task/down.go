package task

import (
	"context"
	"fmt"
	"strings"

	"github.com/loykin/apirun/pkg/env"
)

type Down struct {
	Name    string    `yaml:"name"`
	Auth    string    `yaml:"auth"`
	Env     *env.Env  `yaml:"env"`
	Method  string    `yaml:"method"`
	URL     string    `yaml:"url"`
	Headers []Header  `yaml:"headers"`
	Queries []Query   `yaml:"queries"`
	Body    string    `yaml:"body"`
	Find    *FindSpec `yaml:"find"`
}

// FindSpec is an optional preliminary step for Down execution.
// It allows querying an API (e.g., to discover an ID) and extracting
// variables from the response body into the Down's Local env for templating.
// Any HTTP method supported by RequestSpec.Method can be used.
// If ResponseSpec.ResultCode is empty, all statuses are accepted.
// Only JSON body extraction is supported (like Up.Response).
type FindSpec struct {
	Request  RequestSpec  `yaml:"request"`
	Response ResponseSpec `yaml:"response"`
}

// runFind executes the optional preliminary Find step. On success it merges
// extracted env into d.Env.Local and returns (nil, nil). On validation error
// it returns an ExecResult with the status code and an error. On transport
// errors it returns (nil, error).
func (d *Down) runFind(ctx context.Context) (*ExecResult, error) {
	fhdrs, fqueries, fbody, ferr := d.Find.Request.Render(d.Env)
	if ferr != nil {
		return nil, fmt.Errorf("down.find body template error: %v", ferr)
	}
	fmethod := strings.ToUpper(strings.TrimSpace(d.Find.Request.Method))
	furl := strings.TrimSpace(d.Find.Request.URL)
	if strings.Contains(furl, "{{") {
		furl = d.Env.RenderGoTemplate(furl)
	}
	if fmethod == "" || furl == "" {
		return nil, fmt.Errorf("down.find: method/url not specified")
	}
	freq := buildRequest(ctx, fhdrs, fqueries, fbody)
	fresp, ferr := execByMethod(freq, fmethod, furl)
	if ferr != nil {
		return nil, ferr
	}
	if err := d.Find.Response.ValidateStatus(fresp.StatusCode(), d.Env); err != nil {
		return &ExecResult{StatusCode: fresp.StatusCode(), ExtractedEnv: map[string]string{}}, err
	}
	// Extract and merge env (may error if env_missing=fail)
	extracted, eerr := d.Find.Response.ExtractEnv(fresp.Body())
	if eerr != nil {
		return &ExecResult{StatusCode: fresp.StatusCode(), ExtractedEnv: extracted}, eerr
	}
	if len(extracted) > 0 {
		if d.Env.Local == nil {
			d.Env.Local = env.Map{}
		}
		for k, v := range extracted {
			_ = d.Env.SetString("local", k, v)
		}
	}
	return nil, nil
}

// Execute performs optional Find step then the main HTTP call for Down.
// Any 2xx status on the final call is considered success.
func (d *Down) Execute(ctx context.Context) (*ExecResult, error) {
	// 1) Optional find step
	if d.Find != nil && d.Find.Request.Method != "" && d.Find.Request.URL != "" {
		if res, err := d.runFind(ctx); err != nil {
			// When validation fails we must return an ExecResult with status and error
			return res, err
		}
	}

	// 2) Main down call
	method := strings.ToUpper(strings.TrimSpace(d.Method))
	url := strings.TrimSpace(d.URL)
	if strings.Contains(url, "{{") {
		url = d.Env.RenderGoTemplate(url)
	}
	if method == "" || url == "" {
		return nil, fmt.Errorf("down: method/url not specified")
	}

	hdrs := renderHeaders(d.Env, d.Headers)
	queries := renderQueries(d.Env, d.Queries)
	body, berr := renderBody(d.Env, d.Body)
	if berr != nil {
		return nil, fmt.Errorf("down body template error: %v", berr)
	}

	req := buildRequest(ctx, hdrs, queries, body)
	resp, err := execByMethod(req, method, url)
	if err != nil {
		return nil, err
	}
	status := resp.StatusCode()
	bodyBytes := resp.Body()
	if status < 200 || status >= 300 {
		return &ExecResult{StatusCode: status, ExtractedEnv: map[string]string{}, ResponseBody: string(bodyBytes)}, fmt.Errorf("down failed with status %d", status)
	}
	return &ExecResult{StatusCode: status, ExtractedEnv: map[string]string{}, ResponseBody: string(bodyBytes)}, nil
}
