package util

import (
	"encoding/json"
	"testing"

	"github.com/loykin/apimigrate/internal/env"
)

// FuzzRenderAnyTemplate ensures RenderAnyTemplate never panics on arbitrary JSON-like inputs
// and arbitrary environments. The goal is to exercise walking of maps/slices/strings safely.
func FuzzRenderAnyTemplate(f *testing.F) {
	f.Add([]byte(`{"a":"{{.x}}","b":["{{.y}}",1,true],"c":{"d":"z"}}`), "x", "1")
	f.Add([]byte(`not json`), "x", "1")
	f.Fuzz(func(t *testing.T, data []byte, k, v string) {
		// Limit input size to keep fuzz fast and avoid excessive allocations
		if len(data) > 1<<16 {
			data = data[:1<<16]
		}
		var in interface{}
		_ = json.Unmarshal(data, &in) // if it fails, in stays nil which is fine
		e := env.Env{Global: map[string]string{k: v}}
		_ = RenderAnyTemplate(in, e)
	})
}
