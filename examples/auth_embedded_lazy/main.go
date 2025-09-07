package main

import (
	"context"
	"encoding/base64"
	"fmt"
	"log"
	"net"
	"net/http"
	"path/filepath"
	"sync/atomic"

	"github.com/loykin/apimigrate"
	"github.com/loykin/apimigrate/pkg/env"
)

// This example shows lazy acquisition of multiple embedded auth providers.
// Run:
//
//	go run ./examples/auth_embedded_lazy
func main() {
	ctx := context.Background()

	// Start a small local server with three endpoints requiring different Basic tokens
	var hitsA1, hitsA2, hitsA3 int32
	// Precompute expected Basic headers for each auth
	exp := func(user, pass string) string {
		return "Basic " + base64.StdEncoding.EncodeToString([]byte(user+":"+pass))
	}
	expA1 := exp("u1", "p1")
	expA2 := exp("u2", "p2")
	expA3 := exp("u3", "p3")

	srv := &http.Server{Addr: "127.0.0.1:0"}
	mux := http.NewServeMux()
	mux.HandleFunc("/a1", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != expA1 {
			w.WriteHeader(401)
			_, _ = w.Write([]byte("unauthorized a1"))
			return
		}
		atomic.AddInt32(&hitsA1, 1)
		w.WriteHeader(200)
		_, _ = w.Write([]byte("ok a1"))
	})
	mux.HandleFunc("/a2", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != expA2 {
			w.WriteHeader(401)
			_, _ = w.Write([]byte("unauthorized a2"))
			return
		}
		atomic.AddInt32(&hitsA2, 1)
		w.WriteHeader(200)
		_, _ = w.Write([]byte("ok a2"))
	})
	mux.HandleFunc("/a3", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != expA3 {
			w.WriteHeader(401)
			_, _ = w.Write([]byte("unauthorized a3"))
			return
		}
		atomic.AddInt32(&hitsA3, 1)
		w.WriteHeader(200)
		_, _ = w.Write([]byte("ok a3"))
	})
	srv.Handler = mux
	ln, err := net.Listen("tcp", srv.Addr)
	if err != nil {
		log.Fatalf("listen: %v", err)
	}
	go func() { _ = srv.Serve(ln) }()
	defer func() { _ = srv.Close() }()

	baseURL := "http://" + ln.Addr().String()

	// Prepare migration dir
	migrateDir := "examples/auth_embedded_lazy/migration"

	// Base env with API base
	base := env.Env{Global: env.FromStringMap(map[string]string{"api_base": baseURL})}

	// Define 3 embedded auth providers (Basic) with names a1, a2, a3
	// Using the public wrapper types from apimigrate
	a1 := apimigrate.Auth{Type: apimigrate.AuthTypeBasic, Name: "a1", Methods: apimigrate.BasicAuthConfig{Username: "u1", Password: "p1"}}
	a2 := apimigrate.Auth{Type: apimigrate.AuthTypeBasic, Name: "a2", Methods: apimigrate.BasicAuthConfig{Username: "u2", Password: "p2"}}
	a3 := apimigrate.Auth{Type: apimigrate.AuthTypeBasic, Name: "a3", Methods: apimigrate.BasicAuthConfig{Username: "u3", Password: "p3"}}

	// Open store under example directory
	st, err := apimigrate.OpenStoreFromOptions(migrateDir, nil)
	if err != nil {
		log.Fatalf("open store: %v", err)
	}
	defer func() { _ = st.Close() }()

	storeConfig := apimigrate.StoreConfig{}
	storeConfig.DriverConfig = &apimigrate.SqliteConfig{Path: filepath.Join(migrateDir, apimigrate.StoreDBFileName)}

	m := apimigrate.Migrator{Env: &base, Dir: migrateDir, StoreConfig: &storeConfig}
	m.Auth = []apimigrate.Auth{a1, a2, a3}

	// Run all migrations
	vres, err := m.MigrateUp(ctx, 0)
	if err != nil {
		log.Fatalf("migrate up failed: %v", err)
	}
	for _, vr := range vres {
		if vr != nil && vr.Result != nil {
			fmt.Printf("v%03d: status=%d body=%q\n", vr.Version, vr.Result.StatusCode, vr.Result.ResponseBody)
		}
	}

	fmt.Printf("endpoint hits: a1=%d a2=%d a3=%d\n", hitsA1, hitsA2, hitsA3)
	fmt.Println("lazy auth example completed successfully")
}
