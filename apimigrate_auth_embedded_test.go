package apimigrate

import (
	"context"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync/atomic"
	"testing"
)

// TestEmbeddedAuthAndMigrateUp verifies that acquiring auth via WithName and running MigrateUp
// against the embedded auth example works end-to-end with an httptest server.
func TestEmbeddedAuthAndMigrateUp(t *testing.T) {
	// Arrange
	ctx := context.Background()

	// Prepare a temp sqlite store path to avoid touching repo files
	tmpDir := t.TempDir()
	storePath := filepath.Join(tmpDir, "apimigrate.db")
	ctx = WithStoreOptions(ctx, &StoreOptions{SQLitePath: storePath})

	// Start a local HTTP test server that validates Authorization header
	var hits int32
	expectedAuth := "Basic " + base64.StdEncoding.EncodeToString([]byte("admin:admin"))
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/200" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != expectedAuth {
			t.Fatalf("unexpected Authorization header: got %q want %q", got, expectedAuth)
		}
		atomic.AddInt32(&hits, 1)
		w.WriteHeader(200)
		_, _ = w.Write([]byte("ok"))
	}))
	defer srv.Close()

	// Base env shared with migrations
	base := Env{Global: map[string]string{
		"api_base": srv.URL,
	}}

	// Acquire Basic auth and store under the explicit name used by the migration file
	cfg := BasicAuthConfig{
		Username: "admin",
		Password: "admin",
	}
	if _, _, name, err := AcquireBasicAuthWithName(ctx, "example_basic", cfg); err != nil {
		t.Fatalf("AcquireBasicAuthWithName failed: %v", err)
	} else if name != "example_basic" {
		t.Fatalf("unexpected stored name: %q", name)
	}

	// Act: run migrations from the example directory
	results, err := MigrateUp(ctx, "./examples/auth_embedded/migration", base, 0)

	// Assert
	if err != nil {
		t.Fatalf("MigrateUp error: %v", err)
	}
	if results == nil || len(results) != 1 {
		t.Fatalf("expected exactly 1 migration to run, got %d", len(results))
	}
	if got := atomic.LoadInt32(&hits); got != 1 {
		t.Fatalf("expected server to be hit once, got %d", got)
	}
}
