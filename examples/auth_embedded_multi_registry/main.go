package main

import (
	"context"
	"encoding/base64"
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

	// Use a temporary sqlite store within a temp dir so the example doesn't leave files around
	var storePath string
	tmpDir, err := os.MkdirTemp("", "apimigrate-example-*")
	if err == nil {
		defer func() { _ = os.RemoveAll(tmpDir) }()
		storePath = filepath.Join(tmpDir, "apimigrate.db")
	}

	// Start a local HTTP test server that validates different Authorization headers
	expA := "Basic " + base64.StdEncoding.EncodeToString([]byte("u1:p1"))
	expB := "Basic " + base64.StdEncoding.EncodeToString([]byte("u2:p2"))
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Expect different paths to use different auth headers
		switch r.URL.Path {
		case "/a":
			if got := r.Header.Get("Authorization"); got != expA {
				w.WriteHeader(401)
				_, _ = w.Write([]byte("unauthorized for a"))
				return
			}
		case "/b":
			if got := r.Header.Get("Authorization"); got != expB {
				w.WriteHeader(401)
				_, _ = w.Write([]byte("unauthorized for b"))
				return
			}
		default:
			w.WriteHeader(404)
			return
		}
		w.WriteHeader(200)
		_, _ = w.Write([]byte("ok"))
	}))
	defer srv.Close()

	// Base environment
	base := apimigrate.Env{Global: map[string]string{
		"api_base": srv.URL,
	}}

	// Acquire two different Basic tokens and store under different names
	if _, err := apimigrate.AcquireAuthAndSetEnv(ctx, apimigrate.AuthTypeBasic, "a1", apimigrate.BasicAuthConfig{Username: "u1", Password: "p1"}, &base); err != nil {
		log.Fatalf("acquire auth a1 failed: %v", err)
	}
	if _, err := apimigrate.AcquireAuthAndSetEnv(ctx, apimigrate.AuthTypeBasic, "a2", apimigrate.BasicAuthConfig{Username: "u2", Password: "p2"}, &base); err != nil {
		log.Fatalf("acquire auth a2 failed: %v", err)
	}
	fmt.Println("auth tokens acquired; available as .auth.a1 and .auth.a2")

	// Run migrations from this example's migration directory
	migDir := "./examples/auth_embedded_multi_registry/migration"

	storeConfig := apimigrate.StoreConfig{}
	storeConfig.DriverConfig = &apimigrate.SqliteConfig{Path: storePath}
	m := apimigrate.Migrator{Env: base, Dir: migDir, StoreConfig: &storeConfig}
	if _, err := m.MigrateUp(ctx, 0); err != nil {
		log.Fatalf("migrate up failed: %v", err)
	}
	fmt.Println("multi-registry migrations completed successfully")
}
