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
	var storePath string
	tmpDir, err := os.MkdirTemp("", "apimigrate-example-*")
	if err == nil {
		// best-effort; if creation fails, fall back to default store behavior
		defer func() { _ = os.RemoveAll(tmpDir) }()
		storePath = filepath.Join(tmpDir, "apimigrate.db")
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

	// Programmatically define an auth provider (basic) using struct-based API
	spec := apimigrate.BasicAuthConfig{
		Username: "admin",
		Password: "admin",
	}

	auth := &apimigrate.Auth{Type: apimigrate.AuthTypeBasic, Name: "basic", Methods: map[string]apimigrate.MethodConfig{apimigrate.AuthTypeBasic: spec}}

	// Run migrations from the local directory
	storeConfig := apimigrate.StoreConfig{}
	storeConfig.DriverConfig = &apimigrate.SqliteConfig{Path: storePath}
	m := apimigrate.Migrator{Env: base, Dir: "./examples/auth_embedded/migration", StoreConfig: &storeConfig}
	m.Auth = []apimigrate.Auth{*auth}
	if _, err := m.MigrateUp(ctx, 0); err != nil {
		log.Fatalf("migrate up failed: %v", err)
	}
	fmt.Println("migrations completed successfully")
}
