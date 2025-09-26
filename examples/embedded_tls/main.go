package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"log"
	"path/filepath"
	"runtime"

	"github.com/loykin/apirun"
	"github.com/loykin/apirun/pkg/env"
)

// This example demonstrates how to configure TLS options in embedded mode.
// - InsecureSkipVerify: allow self-signed certificates
// - MinVersion / MaxVersion: restrict allowed TLS versions
// Note: Avoid using InsecureSkipVerify=true in production.
func main() {
	ctx := context.Background()

	// 1) Configure TLS options
	//    Adjust values as needed for your environment.
	cfg := &tls.Config{
		// Set to true to test against a self-signed server
		InsecureSkipVerify: false, // #nosec G402 -- for example purposes only; not recommended in production
		// Specify allowed TLS version range (uncomment if needed)
		// MinVersion: tls.VersionTLS12,
		// MaxVersion: tls.VersionTLS13,
	}

	// 2) Set migration directory
	//    Use a path relative to this source file so it works regardless of run location.
	_, file, _, _ := runtime.Caller(0)
	dir := filepath.Join(filepath.Dir(file), "migration")

	// 3) Prepare base environment
	//    Provide the URL value used by templates.
	baseEnv := env.New()
	_ = baseEnv.SetString("global", "URL", "https://example.com")

	m := apirun.Migrator{
		Dir:              dir,
		Env:              baseEnv,
		SaveResponseBody: false,
		TLSConfig:        cfg,
	}

	res, err := m.MigrateUp(ctx, 0)
	if err != nil {
		log.Fatalf("migrate up error: %v", err)
	}
	for _, r := range res {
		if r != nil && r.Result != nil {
			fmt.Printf("v%03d -> status=%d env=%v\n", r.Version, r.Result.StatusCode, r.Result.ExtractedEnv)
		}
	}
	fmt.Println("migrations completed successfully (embedded_tls)")
}
