package migration

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestRunMigrations_OrderAndSuccess(t *testing.T) {
	// HTTP server records order via header value
	order := 0
	received := []string{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		order++
		received = append(received, r.Header.Get("X-Seq"))
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	dir := t.TempDir()
	// Two migrations out of order by filename insertion but will be ordered by prefix
	// 010_ should run after 001_
	mig1 := "up:\n  name: one\n  env: { }\n  request:\n    headers:\n      - { name: X-Seq, value: \"1\" }\n  response:\n    result_code: [\"200\"]\n"
	mig2 := "up:\n  name: two\n  env: { }\n  request:\n    headers:\n      - { name: X-Seq, value: \"2\" }\n  response:\n    result_code: [\"200\"]\n"
	if err := os.WriteFile(filepath.Join(dir, "010_second.yaml"), []byte(mig2), 0o600); err != nil {
		t.Fatalf("write mig2: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "001_first.yaml"), []byte(mig1), 0o600); err != nil {
		t.Fatalf("write mig1: %v", err)
	}

	res, err := RunMigrations(context.Background(), dir, http.MethodPost, srv.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(res) != 2 {
		t.Fatalf("expected 2 results, got %d", len(res))
	}
	if received[0] != "1" || received[1] != "2" {
		t.Fatalf("expected order [1,2], got %v", received)
	}
}

func TestRunMigrations_StopOnError(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls == 1 {
			w.WriteHeader(500)
			_, _ = w.Write([]byte(`{"err":true}`))
			return
		}
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	dir := t.TempDir()
	mig1 := "up:\n  request: {}\n  response:\n    result_code: [\"200\"]\n"
	mig2 := "up:\n  request: {}\n  response:\n    result_code: [\"200\"]\n"
	_ = os.WriteFile(filepath.Join(dir, "001_first.yaml"), []byte(mig1), 0o600)
	_ = os.WriteFile(filepath.Join(dir, "002_second.yaml"), []byte(mig2), 0o600)

	res, err := RunMigrations(context.Background(), dir, http.MethodGet, srv.URL)
	if err == nil {
		t.Fatalf("expected error on first migration, got nil")
	}
	if len(res) != 1 {
		t.Fatalf("expected 1 result before error, got %d", len(res))
	}
}
