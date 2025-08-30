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

	// Programmatically define an auth provider (basic) using AcquireAuthAndSetEnv
	spec := apimigrate.BasicAuthConfig{
		Username: "admin",
		Password: "admin",
	}

	// Acquire token and store under .auth.basic for template access
	if _, err := apimigrate.AcquireAuthAndSetEnv(ctx, apimigrate.AuthTypeBasic, "basic", spec, &base); err != nil {
		log.Fatalf("acquire auth failed: %v", err)
	} else {
		fmt.Printf("auth token acquired; available as .auth.basic\n")
	}

	// Run migrations from the local directory
	st, err := apimigrate.OpenStoreFromOptions("./examples/auth_embedded/migration", &apimigrate.StoreOptions{SQLitePath: storePath})
	if err != nil {
		log.Fatalf("open store failed: %v", err)
	}
	defer func() { _ = st.Close() }()
	m := apimigrate.Migrator{Env: base, Dir: "./examples/auth_embedded/migration", Store: *st}
	if _, err := m.MigrateUp(ctx, 0); err != nil {
		log.Fatalf("migrate up failed: %v", err)
	}
	fmt.Println("migrations completed successfully")
}
