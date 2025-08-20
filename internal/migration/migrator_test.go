package migration

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDecodeTaskYAML_Valid(t *testing.T) {
	// Minimal but representative YAML including Up and Down
	yaml := strings.NewReader(
		"up:\n  name: test-up\n  env: { X: 'y' }\n  request:\n    method: GET\n    url: http://example.com\n  response:\n    result_code: ['200']\n" +
			"down:\n  name: test-down\n  env: {}\n  method: DELETE\n  url: http://example.com\n",
	)

	tk, err := decodeTaskYAML(yaml)
	if err != nil {
		t.Fatalf("unexpected error decoding: %v", err)
	}
	// Basic assertions
	if tk.Up.Name != "test-up" {
		t.Fatalf("expected up.name 'test-up', got %q", tk.Up.Name)
	}
	if tk.Up.Request.Method != http.MethodGet || tk.Up.Request.URL != "http://example.com" {
		t.Fatalf("unexpected up.request: method=%q url=%q", tk.Up.Request.Method, tk.Up.Request.URL)
	}
	if len(tk.Up.Response.ResultCode) != 1 || tk.Up.Response.ResultCode[0] != "200" {
		t.Fatalf("unexpected up.response result_code: %v", tk.Up.Response.ResultCode)
	}
	if tk.Down.Name != "test-down" || tk.Down.Method != http.MethodDelete || tk.Down.URL != "http://example.com" {
		t.Fatalf("unexpected down fields: %+v", tk.Down)
	}
}

func TestDecodeTaskYAML_Invalid(t *testing.T) {
	bad := strings.NewReader(":: not yaml ::")
	_, err := decodeTaskYAML(bad)
	if err == nil {
		t.Fatalf("expected error for invalid yaml, got nil")
	}
}

func TestLoadTaskFromFile_Success(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "001_sample.yaml")
	content := "up:\n  request: { method: POST, url: http://localhost }\n  response: { result_code: ['200'] }\n"
	if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}

	tk, err := loadTaskFromFile(p)
	if err != nil {
		t.Fatalf("unexpected error loading from file: %v", err)
	}
	if strings.ToUpper(tk.Up.Request.Method) != http.MethodPost {
		t.Fatalf("expected method POST, got %q", tk.Up.Request.Method)
	}
}

func TestLoadTaskFromFile_NotFound(t *testing.T) {
	_, err := loadTaskFromFile(filepath.Join(t.TempDir(), "missing.yaml"))
	if err == nil {
		t.Fatalf("expected error for missing file, got nil")
	}
}
