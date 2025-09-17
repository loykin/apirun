package main

import (
	"context"
	"fmt"
	"log"

	"github.com/loykin/apimigrate"
	"github.com/loykin/apimigrate/pkg/env"
)

// Example demonstrating structured logging with apimigrate
func main() {
	// Initialize structured logging at debug level to see all log messages
	logger := apimigrate.NewLogger(apimigrate.LogLevelDebug)
	apimigrate.SetDefaultLogger(logger)

	logger.Info("apimigrate example with structured logging started")

	// Create a base environment for migrations
	base := env.New()
	_ = base.SetString("global", "api_base", "https://httpbin.org")

	ctx := context.Background()

	// Configure migrator with logging
	m := apimigrate.Migrator{
		Dir: "examples/embedded_logging/migration",
		Env: base,
	}

	logger.Info("running migrations with structured logging",
		"dir", m.Dir,
		"env_count", len(base.Global))

	// The migration execution will now include detailed structured logging
	results, err := m.MigrateUp(ctx, 0)
	if err != nil {
		// Log the error with structured context
		logger.Error("migration failed", err,
			"dir", m.Dir,
			"results_count", len(results))
		log.Fatalf("Migration failed: %v", err)
	}

	// Log successful completion with results summary
	logger.Info("migrations completed successfully",
		"total_migrations", len(results),
		"dir", m.Dir)

	for _, result := range results {
		if result != nil && result.Result != nil {
			// Structured logging of each migration result
			logger.Info("migration result",
				"version", result.Version,
				"status_code", result.Result.StatusCode,
				"extracted_env_count", len(result.Result.ExtractedEnv))

			fmt.Printf("v%03d: status=%d env=%v\n",
				result.Version,
				result.Result.StatusCode,
				result.Result.ExtractedEnv)
		}
	}

	logger.Info("apimigrate example with structured logging completed")
}
