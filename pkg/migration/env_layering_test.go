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

// Verifies that Global env from config persists across tasks and Local env is reset per task.
func TestRunMigrations_GlobalPersists_LocalPerTask(t *testing.T) {
	// Server validates header X-Realm (from Global) and query idx (from Local)
	var received []string
	var seenRealms []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received = append(received, r.URL.Query().Get("idx"))
		seenRealms = append(seenRealms, r.Header.Get("X-Realm"))
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	dir := t.TempDir()
	mig1 := "up:\n  name: one\n  env: { idx: '1' }\n  request:\n    method: POST\n    url: " + srv.URL + "\n    headers:\n      - { name: X-Realm, value: '{{.realm}}' }\n    queries:\n      - { name: idx, value: '{{.idx}}' }\n  response:\n    result_code: ['200']\n"
	mig2 := "up:\n  name: two\n  env: { idx: '2' }\n  request:\n    method: POST\n    url: " + srv.URL + "\n    headers:\n      - { name: X-Realm, value: '{{.realm}}' }\n    queries:\n      - { name: idx, value: '{{.idx}}' }\n  response:\n    result_code: ['200']\n"
	if err := os.WriteFile(filepath.Join(dir, "001_first.yaml"), []byte(mig1), 0o600); err != nil {
		t.Fatalf("write mig1: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "002_second.yaml"), []byte(mig2), 0o600); err != nil {
		t.Fatalf("write mig2: %v", err)
	}

	base := env.Env{Global: map[string]string{"realm": "sample"}}
	res, err := RunMigrationsWithEnv(context.Background(), dir, "", "", base)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(res) != 2 {
		t.Fatalf("expected 2 results, got %d", len(res))
	}
	// Local per task
	if received[0] != "1" || received[1] != "2" {
		t.Fatalf("expected idx [1 2], got %v", received)
	}
	// Global constant across tasks
	if seenRealms[0] != "sample" || seenRealms[1] != "sample" {
		t.Fatalf("expected realm header to persist as 'sample', got %v", seenRealms)
	}
}
