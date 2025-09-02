package migration

import (
	"bytes"
	"testing"

	"github.com/loykin/apimigrate/internal/task"
)

// FuzzDecodeTaskYAML ensures the YAML decoder for Task never panics on arbitrary input.
func FuzzDecodeTaskYAML(f *testing.F) {
	f.Add([]byte("up:\n  name: x\n"))
	f.Add([]byte("not: yaml"))
	f.Add([]byte("up:\n  request:\n    method: GET\n    url: http://example.com\n"))

	f.Fuzz(func(t *testing.T, b []byte) {
		// Limit size to keep it fast
		if len(b) > 1<<16 {
			b = b[:1<<16]
		}
		var tk task.Task
		_ = tk.DecodeYAML(bytes.NewReader(b))
	})
}
