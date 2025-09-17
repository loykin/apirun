package main

import (
	"fmt"
	"time"

	"github.com/loykin/apimigrate"
)

func main() {
	fmt.Println("=== Color Logging Demo ===")

	// Test different log formats
	formats := []struct {
		name   string
		logger *apimigrate.Logger
	}{
		{"Plain Text", apimigrate.NewLogger(apimigrate.LogLevelInfo)},
		{"JSON", apimigrate.NewJSONLogger(apimigrate.LogLevelInfo)},
		{"Color", apimigrate.NewColorLogger(apimigrate.LogLevelInfo)},
	}

	for _, f := range formats {
		fmt.Printf("\n--- %s Logger ---\n", f.name)

		// Test different log levels
		f.logger.Debug("This is a debug message", "component", "test")
		f.logger.Info("Migration started",
			"version", 1,
			"file", "001_create_users.yaml",
			"dry_run", false)

		f.logger.Info("HTTP request completed",
			"method", "POST",
			"url", "https://api.example.com/users",
			"status_code", 201,
			"duration_ms", 234,
			"success", true)

		f.logger.Warn("Authentication token will expire soon",
			"expires_in", "5m30s",
			"token_type", "Bearer")

		f.logger.Error("Migration failed",
			"error", "connection timeout",
			"retry_count", 3,
			"failed", true)

		// Test sensitive data masking
		f.logger.Info("User authentication",
			"username", "admin",
			"password", "secret123",
			"api_key", "sk_test_1234567890")

		// Simulate some delay for visual separation
		time.Sleep(100 * time.Millisecond)
	}

	fmt.Println("\n=== Demo Complete ===")
	fmt.Println("Notice how:")
	fmt.Println("✅ Color format highlights different log levels")
	fmt.Println("✅ Error messages are in red, success in green")
	fmt.Println("✅ Component names are highlighted")
	fmt.Println("✅ Sensitive data is automatically masked")
	fmt.Println("✅ Different data types have different colors")
}
