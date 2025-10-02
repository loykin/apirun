package main

import (
	"time"

	"github.com/loykin/apirun/internal/common"
)

func main() {
	// Create a color logger with aligned output
	logger := common.NewColorLogger(common.LogLevelInfo)

	// Example logs similar to your monitoring output
	logger.Info("Starting migration", "step", 1, "name", "Create user account")
	logger.Info("HTTP request", "method", "POST", "url", "https://api.example.com/users")

	// Simulate some delay
	time.Sleep(890 * time.Millisecond)

	logger.Info("HTTP response", "status", 201, "duration", "890ms", "success", true)
	logger.Info("Migration completed", "step", 1, "success", true, "duration", "1.2s")

	// Additional examples with different log levels
	logger.Debug("Debug information", "component", "migrator", "version", 1)
	logger.Warn("Warning message", "auth", "basic", "store", "postgresql")
	logger.Error("Error occurred", "error", "connection timeout", "duration", "30s")

	// Examples with longer values to show alignment
	logger.Info("Processing step", "step", 2, "name", "Update user permissions with complex validation")
	logger.Info("Database query", "method", "SELECT", "url", "postgresql://localhost:5432/apirun_db?sslmode=disable", "duration", "45ms")
}
