package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"

	"github.com/loykin/apirun"
	"github.com/loykin/apirun/pkg/env"
)

func main() {
	ctx := context.Background()

	// Use a temporary sqlite store, avoiding writing example DB files into the repo
	var storePath string
	tmpDir, err := os.MkdirTemp("", "apirun-example-*")
	if err == nil {
		// best-effort; if creation fails, fall back to default store behavior
		defer func() { _ = os.RemoveAll(tmpDir) }()
		storePath = filepath.Join(tmpDir, "apirun.db")
	}

	// Start a local HTTP test server to avoid external network dependency
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		_, _ = w.Write([]byte("ok"))
	}))
	defer srv.Close()

	// Base environment available to all migrations
	base := env.Env{Global: env.FromStringMap(map[string]string{
		"api_base": srv.URL,
	})}

	// Programmatically define an auth provider (basic) using struct-based API
	spec := apirun.BasicAuthConfig{
		Username: "admin",
		Password: "admin",
	}

	auth := &apirun.Auth{Type: apirun.AuthTypeBasic, Name: "basic", Methods: spec}

	// Run migrations from the local directory
	storeConfig := apirun.StoreConfig{}
	storeConfig.DriverConfig = &apirun.SqliteConfig{Path: storePath}
	m := apirun.Migrator{Env: &base, Dir: "./examples/auth_embedded/migration", StoreConfig: &storeConfig}
	m.Auth = []apirun.Auth{*auth}
	if _, err := m.MigrateUp(ctx, 0); err != nil {
		log.Fatalf("migrate up failed: %v", err)
	}
	fmt.Println("migrations completed successfully")
}
