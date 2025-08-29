package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"

	"github.com/loykin/apimigrate"
)

func main() {
	ctx := context.Background()

	// Use a temporary sqlite store, avoiding writing example DB files into the repo
	tmpDir, err := os.MkdirTemp("", "apimigrate-example-*")
	if err == nil {
		// best-effort; if creation fails, fall back to default store behavior
		defer func() { _ = os.RemoveAll(tmpDir) }()
		storePath := filepath.Join(tmpDir, "apimigrate.db")
		ctx = apimigrate.WithStoreOptions(ctx, &apimigrate.StoreOptions{SQLitePath: storePath})
	}

	// Start a local HTTP test server to avoid external network dependency
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		_, _ = w.Write([]byte("ok"))
	}))
	defer srv.Close()

	// Base environment available to all migrations
	base := apimigrate.Env{Global: map[string]string{
		"api_base": srv.URL,
	}}

	// Programmatically define an auth provider (basic) using the WithName helper
	cfg := apimigrate.BasicAuthConfig{
		// Name is intentionally omitted; we use an explicit logical name below
		Username: "admin",
		Password: "admin",
	}

	// Acquire token and store under an explicit logical name for use by migrations
	if _, _, name, err := apimigrate.AcquireBasicAuthWithName(ctx, "example_basic", cfg); err != nil {
		log.Fatalf("acquire auth failed: %v", err)
	} else {
		fmt.Printf("auth provider %q is ready\n", name)
	}

	// Run migrations from the local directory
	if _, err := apimigrate.MigrateUp(ctx, "./examples/auth_embedded/migration", base, 0); err != nil {
		log.Fatalf("migrate up failed: %v", err)
	}
	fmt.Println("migrations completed successfully")
}
