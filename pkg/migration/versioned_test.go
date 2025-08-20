package migration

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/loykin/apimigrate/pkg/env"
)

// Ensures that MigrateDown calls Task.DownExecute (delegation) and rolls back in reverse order.
func TestMigrateDown_DelegatesAndReverseOrder(t *testing.T) {
	var received []string
	// Server records X-Seq header for down requests
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// For both up and down we just return 200; we only record when header present
		if v := r.Header.Get("X-Seq"); v != "" {
			received = append(received, v)
		}
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	dir := t.TempDir()
	// Create two migrations that have both up and down sections.
	// Up sections will be used to apply; Down sections will be used to rollback.
	mig1 := "up:\n  name: one\n  env: { }\n  request:\n    method: POST\n    url: " + srv.URL + "\n  response:\n    result_code: [\"200\"]\n" +
		"down:\n  name: one-down\n  env: { }\n  method: DELETE\n  url: " + srv.URL + "\n  headers:\n    - { name: X-Seq, value: \"1\" }\n"
	mig2 := "up:\n  name: two\n  env: { }\n  request:\n    method: POST\n    url: " + srv.URL + "\n  response:\n    result_code: [\"200\"]\n" +
		"down:\n  name: two-down\n  env: { }\n  method: DELETE\n  url: " + srv.URL + "\n  headers:\n    - { name: X-Seq, value: \"2\" }\n"
	if err := os.WriteFile(filepath.Join(dir, "001_first.yaml"), []byte(mig1), 0o600); err != nil {
		t.Fatalf("write mig1: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "002_second.yaml"), []byte(mig2), 0o600); err != nil {
		t.Fatalf("write mig2: %v", err)
	}

	ctx := context.Background()
	base := env.Env{Global: map[string]string{}}
	// Apply both
	if _, err := MigrateUp(ctx, dir, base, 0); err != nil {
		t.Fatalf("migrate up failed: %v", err)
	}
	// Rollback to 0 (all downs), expect order 2 then 1
	res, err := MigrateDown(ctx, dir, base, 0)
	if err != nil {
		t.Fatalf("migrate down failed: %v", err)
	}
	if len(res) != 2 {
		t.Fatalf("expected 2 down results, got %d", len(res))
	}
	if len(received) != 2 || received[0] != "2" || received[1] != "1" {
		t.Fatalf("expected reverse order [2,1], got %v", received)
	}
}
