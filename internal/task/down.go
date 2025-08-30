package task

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/go-resty/resty/v2"
	"github.com/loykin/apimigrate/internal/env"
	"github.com/loykin/apimigrate/internal/httpc"
)

type Down struct {
	Name    string    `yaml:"name"`
	Auth    string    `yaml:"auth"`
	Env     env.Env   `yaml:"env"`
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

// Execute performs optional Find step then the main HTTP call for Down.
// Any 2xx status on the final call is considered success.
func (d Down) Execute(ctx context.Context) (*ExecResult, error) {
	// 1) Optional find step
	if d.Find != nil {
		if res, err := runFind(ctx, &d); err != nil {
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
	body := renderBody(d.Env, d.Body)

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

// runFind executes the optional preliminary Find step. On success it merges
// extracted env into d.Env.Local and returns (nil, nil). On validation error
// it returns an ExecResult with the status code and an error. On transport
// errors it returns (nil, error).
func runFind(ctx context.Context, d *Down) (*ExecResult, error) {
	fhdrs, fqueries, fbody := d.Find.Request.Render(d.Env)
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
	// Extract and merge env
	extracted := d.Find.Response.ExtractEnv(fresp.Body())
	if len(extracted) > 0 {
		if d.Env.Local == nil {
			d.Env.Local = map[string]string{}
		}
		for k, v := range extracted {
			d.Env.Local[k] = v
		}
	}
	return nil, nil
}

func renderHeaders(e env.Env, hs []Header) map[string]string {
	hdrs := make(map[string]string)
	for _, h := range hs {
		if h.Name == "" {
			continue
		}
		val := h.Value
		if strings.Contains(val, "{{") {
			val = e.RenderGoTemplate(val)
		}
		hdrs[h.Name] = val
	}
	return hdrs
}

func renderQueries(e env.Env, qs []Query) map[string]string {
	m := make(map[string]string)
	for _, q := range qs {
		if q.Name == "" {
			continue
		}
		val := q.Value
		if strings.Contains(val, "{{") {
			val = e.RenderGoTemplate(val)
		}
		m[q.Name] = val
	}
	return m
}

func renderBody(e env.Env, b string) string {
	if strings.Contains(b, "{{") {
		return e.RenderGoTemplate(b)
	}
	return b
}

func buildRequest(ctx context.Context, headers map[string]string, queries map[string]string, body string) *resty.Request {
	client := httpc.New(ctx)
	req := client.R().SetContext(ctx).SetHeaders(headers).SetQueryParams(queries)
	if strings.TrimSpace(body) != "" {
		if isJSON(body) {
			req.SetHeader("Content-Type", "application/json")
			req.SetBody([]byte(body))
		} else {
			req.SetBody(body)
		}
	}
	return req
}

func execByMethod(req *resty.Request, method, url string) (*resty.Response, error) {
	switch method {
	case http.MethodGet:
		return req.Get(url)
	case http.MethodPost:
		return req.Post(url)
	case http.MethodPut:
		return req.Put(url)
	case http.MethodPatch:
		return req.Patch(url)
	case http.MethodDelete:
		return req.Delete(url)
	default:
		return nil, fmt.Errorf("down.find: unsupported method: %s", method)
	}
}
