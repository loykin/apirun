package task

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/go-resty/resty/v2"
	auth "github.com/loykin/apimigrate/internal/auth"
	env "github.com/loykin/apimigrate/internal/env"
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
		// Render request parts using current env (Global+Local)
		fhdrs, fqueries, fbody := d.Find.Request.Render(d.Env)
		fmethod := strings.ToUpper(strings.TrimSpace(d.Find.Request.Method))
		furl := strings.TrimSpace(d.Find.Request.URL)
		// Render Find URL if templated
		if strings.Contains(furl, "{{") {
			furl = d.Env.RenderGoTemplate(furl)
		}
		if fmethod == "" || furl == "" {
			return nil, fmt.Errorf("down.find: method/url not specified")
		}
		client := httpc.New(ctx)
		freq := client.R().SetContext(ctx).SetHeaders(fhdrs).SetQueryParams(fqueries)
		if strings.TrimSpace(fbody) != "" {
			if isJSON(fbody) {
				freq.SetHeader("Content-Type", "application/json")
				freq.SetBody([]byte(fbody))
			} else {
				freq.SetBody(fbody)
			}
		}
		var fresp *resty.Response
		var ferr error
		switch fmethod {
		case http.MethodGet:
			fresp, ferr = freq.Get(furl)
		case http.MethodPost:
			fresp, ferr = freq.Post(furl)
		case http.MethodPut:
			fresp, ferr = freq.Put(furl)
		case http.MethodPatch:
			fresp, ferr = freq.Patch(furl)
		case http.MethodDelete:
			fresp, ferr = freq.Delete(furl)
		default:
			return nil, fmt.Errorf("down.find: unsupported method: %s", fmethod)
		}
		if ferr != nil {
			return nil, ferr
		}
		// Validate allowed status if provided
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
	}

	// 2) Main down call
	method := strings.ToUpper(strings.TrimSpace(d.Method))
	url := strings.TrimSpace(d.URL)
	// Render main URL if templated
	if strings.Contains(url, "{{") {
		url = d.Env.RenderGoTemplate(url)
	}
	if method == "" || url == "" {
		return nil, fmt.Errorf("down: method/url not specified")
	}

	hdrs := make(map[string]string)
	for _, h := range d.Headers {
		if h.Name == "" {
			continue
		}
		val := h.Value
		if strings.Contains(val, "{{") {
			val = d.Env.RenderGoTemplate(val)
		}
		hdrs[h.Name] = val
	}
	// Inject auth if configured
	if strings.TrimSpace(d.Auth) != "" {
		if h, v, ok := auth.GetToken(d.Auth); ok {
			if _, exists := hdrs[h]; !exists {
				hdrs[h] = v
			}
		} else {
			key := strings.ToUpper(d.Auth)
			if v, ok := d.Env.Lookup(key); ok {
				if _, exists := hdrs["Authorization"]; !exists {
					hdrs["Authorization"] = v
				}
			}
		}
	}

	queries := make(map[string]string)
	for _, q := range d.Queries {
		if q.Name == "" {
			continue
		}
		val := q.Value
		if strings.Contains(val, "{{") {
			val = d.Env.RenderGoTemplate(val)
		}
		queries[q.Name] = val
	}

	body := d.Body
	if strings.Contains(body, "{{") {
		body = d.Env.RenderGoTemplate(body)
	}

	client := httpc.New(ctx)
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
	switch method {
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
	bodyBytes := resp.Body()
	if status < 200 || status >= 300 {
		return &ExecResult{StatusCode: status, ExtractedEnv: map[string]string{}, ResponseBody: string(bodyBytes)}, fmt.Errorf("down failed with status %d", status)
	}
	return &ExecResult{StatusCode: status, ExtractedEnv: map[string]string{}, ResponseBody: string(bodyBytes)}, nil
}
