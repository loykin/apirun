package main

import (
	"context"
	"fmt"
	"log"
	"path/filepath"

	"github.com/loykin/apirun"
	"github.com/loykin/apirun/pkg/env"
)

func main() {
	// Base environment available to all migrations
	base := env.Env{Global: env.FromStringMap(map[string]string{
		"service": "embedded-sample",
	})}

	ctx := context.Background()
	// Resolve migrations directory relative to this example
	dir := filepath.Join("examples", "embedded", "migration")

	fmt.Printf("running embedded migrations in %s...\n", dir)
	m := apirun.Migrator{Env: &base, Dir: dir}
	vres, err := m.MigrateUp(ctx, 0)
	if err != nil {
		// Print partial results if any
		if len(vres) > 0 {
			for _, vr := range vres {
				if vr != nil && vr.Result != nil {
					log.Printf("migration v%03d: status=%d env=%v", vr.Version, vr.Result.StatusCode, vr.Result.ExtractedEnv)
				}
			}
		}
		log.Fatalf("migrate up failed: %v", err)
	}

	for _, vr := range vres {
		if vr == nil || vr.Result == nil {
			continue
		}
		fmt.Printf("migration v%03d done: status=%d, extracted=%v\n", vr.Version, vr.Result.StatusCode, vr.Result.ExtractedEnv)
	}
	fmt.Println("embedded migrations completed successfully")
}
