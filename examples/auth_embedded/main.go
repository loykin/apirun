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

	// Programmatically define an auth provider (basic) using AcquireAuthAndSetEnv
	spec := apimigrate.BasicAuthConfig{
		Username: "admin",
		Password: "admin",
	}

	// Acquire token, store under a logical name (optional), and inject into base env as _auth_token
	if v, err := apimigrate.AcquireAuthAndSetEnv(ctx, apimigrate.AuthTypeBasic, "example_basic", spec, &base); err != nil {
		log.Fatalf("acquire auth failed: %v", err)
	} else {
		fmt.Printf("auth token acquired and injected (_auth_token): %q\n", v)
	}

	// Run migrations from the local directory
	if _, err := apimigrate.MigrateUp(ctx, "./examples/auth_embedded/migration", base, 0); err != nil {
		log.Fatalf("migrate up failed: %v", err)
	}
	fmt.Println("migrations completed successfully")
}
