package main

import (
	"fmt"

	"github.com/loykin/apimigrate"
)

func main() {
	// Create a logger with masking enabled
	logger := apimigrate.NewLogger(apimigrate.LogLevelInfo)

	fmt.Println("=== Testing Sensitive Data Masking ===")

	// Test 1: Log with password (should be masked)
	logger.Info("user authentication",
		"username", "admin",
		"password", "secret123",
		"api_key", "sk_test_1234567890")

	// Test 2: Log with authorization header (should be masked)
	logger.Info("HTTP request",
		"method", "POST",
		"url", "/api/users",
		"authorization", "Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9")

	// Test 3: Normal data (should not be masked)
	logger.Info("user data",
		"username", "john.doe",
		"email", "john@example.com",
		"role", "admin")

	// Test 4: Test disabling masking
	logger.EnableMasking(false)
	logger.Info("masking disabled - sensitive data visible",
		"password", "this_will_be_visible",
		"token", "visible_token")

	// Test 5: Re-enable masking
	logger.EnableMasking(true)
	logger.Info("masking re-enabled - sensitive data hidden",
		"password", "this_will_be_masked",
		"client_secret", "hidden_secret")

	// Test 6: Test global masking function
	sensitiveText := `{"username": "admin", "password": "secret123", "api_key": "sk_test_1234"}`
	masked := apimigrate.MaskSensitiveData(sensitiveText)
	fmt.Printf("\nOriginal: %s\n", sensitiveText)
	fmt.Printf("Masked:   %s\n", masked)

	// Test 7: Custom masking patterns
	masker := apimigrate.NewMasker()
	customPattern := apimigrate.SensitivePattern{
		Name: "custom_token",
		Keys: []string{"custom_token", "my_secret"},
	}
	masker.AddPattern(customPattern)

	logger.SetMasker(masker)
	logger.Info("custom pattern test",
		"custom_token", "this_should_be_masked",
		"my_secret", "another_secret",
		"normal_field", "visible_data")

	fmt.Println("\n=== Masking Demo Complete ===")
}
