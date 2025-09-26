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

	"github.com/loykin/apirun"
	"github.com/loykin/apirun/pkg/env"
)

// This example shows the decoupled flow: acquire auth first, then run migrations.
func main() {
	ctx := context.Background()

	// Temporary sqlite store in a temp dir
	var storePath string
	tmpDir, err := os.MkdirTemp("", "apirun-example-*")
	if err == nil {
		defer func() { _ = os.RemoveAll(tmpDir) }()
		storePath = filepath.Join(tmpDir, "apirun.db")
	}

	// Local HTTP server that expects different Basic headers per path
	expA := "Basic " + base64.StdEncoding.EncodeToString([]byte("u1:p1"))
	expB := "Basic " + base64.StdEncoding.EncodeToString([]byte("u2:p2"))
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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

	// Base env
	base := env.Env{Global: env.FromStringMap(map[string]string{
		"api_base": srv.URL,
	})}

	// 1) Acquire tokens separately (decoupled from migrator). Store into base.Auth.
	// a1
	specA := apirun.BasicAuthConfig{Username: "u1", Password: "p1"}
	authA := &apirun.Auth{Type: apirun.AuthTypeBasic, Name: "a1", Methods: specA}
	if v, err := authA.Acquire(ctx, &base); err != nil {
		log.Fatalf("acquire a1 failed: %v", err)
	} else {
		if base.Auth == nil {
			base.Auth = env.Map{}
		}
		base.Auth["a1"] = env.Str(v)
	}
	// a2
	specB := apirun.BasicAuthConfig{Username: "u2", Password: "p2"}
	authB := &apirun.Auth{Type: apirun.AuthTypeBasic, Name: "a2", Methods: specB}
	if v, err := authB.Acquire(ctx, &base); err != nil {
		log.Fatalf("acquire a2 failed: %v", err)
	} else {
		if base.Auth == nil {
			base.Auth = env.Map{}
		}
		base.Auth["a2"] = env.Str(v)
	}
	fmt.Println("auth tokens acquired separately; available as .auth.a1 and .auth.a2")

	// 2) Run migrations (migrator.Auth left empty to demonstrate separation)
	migDir := "./examples/auth_embedded_multi_registry_type2/migration"
	storeConfig := apirun.StoreConfig{}
	storeConfig.DriverConfig = &apirun.SqliteConfig{Path: storePath}
	m := apirun.Migrator{Env: &base, Dir: migDir, StoreConfig: &storeConfig}
	if _, err := m.MigrateUp(ctx, 0); err != nil {
		log.Fatalf("migrate up failed: %v", err)
	}
	fmt.Println("decoupled multi-registry migrations completed successfully")
}
