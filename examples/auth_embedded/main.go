package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"

	"github.com/loykin/apimigrate"
)

func main() {
	ctx := context.Background()

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

	// Programmatically define an auth provider (basic) using the public wrapper
	cfg := apimigrate.BasicAuthConfig{
		Name:     "example_basic",
		Username: "admin",
		Password: "admin",
	}

	// Acquire token and store under logical name for use by migrations
	if _, _, name, err := apimigrate.AcquireBasicAuth(ctx, cfg); err != nil {
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
