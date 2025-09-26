package main

import (
	"context"
	"fmt"

	"github.com/loykin/apirun"
	"github.com/loykin/apirun/pkg/env"
)

func main() {
	fmt.Println("=== Testing Enhanced Migration Logging ===")

	ctx := context.Background()

	// Create a simple migrator (this won't actually run migrations but will show logging)
	m := apirun.Migrator{
		Dir: "./migration", // This directory doesn't exist, that's OK for demo
		Env: &env.Env{
			Global: env.FromStringMap(map[string]string{"demo": "value"}),
		},
	}

	// This will fail but will demonstrate the enhanced logging
	fmt.Println("\n1. Testing migration up logging...")
	results, err := m.MigrateUp(ctx, 0)
	if err != nil {
		fmt.Printf("Expected error (no migration dir): %v\n", err)
	}
	fmt.Printf("Results count: %d\n", len(results))

	fmt.Println("\n2. Testing migration down logging...")
	results, err = m.MigrateDown(ctx, 0)
	if err != nil {
		fmt.Printf("Expected error (no migration dir): %v\n", err)
	}
	fmt.Printf("Results count: %d\n", len(results))

	fmt.Println("\n=== Enhanced Logging Demo Complete ===")
	fmt.Println("Check the log output above to see:")
	fmt.Println("- Migration start/completion times with durations")
	fmt.Println("- Applied/rolled back migration counts")
	fmt.Println("- Individual migration progress")
}
