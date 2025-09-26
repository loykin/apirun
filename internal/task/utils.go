package task

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/go-resty/resty/v2"
	"github.com/loykin/apirun/internal/httpc"
	"github.com/loykin/apirun/pkg/env"
)

func isJSON(s string) bool {
	t := strings.TrimSpace(s)
	if t == "" {
		return false
	}
	if (strings.HasPrefix(t, "{") && strings.HasSuffix(t, "}")) || (strings.HasPrefix(t, "[") && strings.HasSuffix(t, "]")) {
		var js json.RawMessage
		return json.Unmarshal([]byte(t), &js) == nil
	}
	return false
}

func anyToString(v interface{}) string {
	switch val := v.(type) {
	case string:
		return val
	case fmt.Stringer:
		return val.String()
	case float64:
		// Avoid scientific notation for integers
		if val == float64(int64(val)) {
			return fmt.Sprintf("%d", int64(val))
		}
		return fmt.Sprintf("%v", val)
	case bool:
		if val {
			return "true"
		}
		return "false"
	case nil:
		return ""
	default:
		// Fallback to JSON
		b, err := json.Marshal(val)
		if err != nil {
			return fmt.Sprintf("%v", val)
		}
		// Remove surrounding quotes if it's a basic JSON string
		b = bytes.TrimSpace(b)
		if len(b) >= 2 && b[0] == '"' && b[len(b)-1] == '"' {
			return string(b[1 : len(b)-1])
		}
		return string(b)
	}
}

func renderHeaders(e *env.Env, hs []Header) map[string]string {
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

func renderQueries(e *env.Env, qs []Query) map[string]string {
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

func renderBody(e *env.Env, b string) (string, error) {
	if strings.Contains(b, "{{") {
		return e.RenderGoTemplateErr(b)
	}
	return b, nil
}

var tlsConfig *tls.Config

// SetTLSConfig configures the TLS settings used by HTTP requests within tasks.
// Passing nil resets to default client behavior.
func SetTLSConfig(cfg *tls.Config) {
	tlsConfig = cfg
}

func buildRequest(ctx context.Context, headers map[string]string, queries map[string]string, body string) *resty.Request {
	h := httpc.Httpc{TlsConfig: tlsConfig}
	client := h.New()
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
