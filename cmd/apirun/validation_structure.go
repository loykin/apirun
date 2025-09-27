package main

import (
	"fmt"
	"strings"
)

// validateMigrationStructure validates the overall structure of a migration file
func validateMigrationStructure(migration map[string]interface{}, result *ValidationResult) {
	// Check for required 'up' section
	up, hasUp := migration["up"]
	if !hasUp {
		result.Errors = append(result.Errors, "Missing required 'up' section")
		return
	}

	upMap, ok := up.(map[string]interface{})
	if !ok {
		result.Errors = append(result.Errors, "'up' section must be a map/object")
		return
	}

	// Validate up section
	validateUpSection(upMap, result)

	// Check for optional 'down' section
	if down, hasDown := migration["down"]; hasDown {
		downMap, ok := down.(map[string]interface{})
		if !ok {
			result.Errors = append(result.Errors, "'down' section must be a map/object")
		} else {
			validateDownSection(downMap, result)
		}
	} else {
		result.Warnings = append(result.Warnings, "No 'down' section found - consider adding for rollback capability")
	}

	// Check for unexpected root level keys
	allowedKeys := map[string]bool{
		"up":   true,
		"down": true,
	}

	for key := range migration {
		if !allowedKeys[key] {
			result.Warnings = append(result.Warnings, fmt.Sprintf("Unexpected root level key: '%s'", key))
		}
	}
}

// validateUpSection validates the 'up' section of a migration
func validateUpSection(up map[string]interface{}, result *ValidationResult) {
	// Check for required fields
	requiredFields := []string{"name"}
	for _, field := range requiredFields {
		if _, exists := up[field]; !exists {
			result.Errors = append(result.Errors, fmt.Sprintf("Missing required field in 'up' section: '%s'", field))
		}
	}

	// Validate name field
	if name, exists := up["name"]; exists {
		if nameStr, ok := name.(string); !ok {
			result.Errors = append(result.Errors, "'name' field must be a string")
		} else if strings.TrimSpace(nameStr) == "" {
			result.Errors = append(result.Errors, "'name' field cannot be empty")
		}
	}

	// Validate env field (optional)
	if env, exists := up["env"]; exists {
		if _, ok := env.(map[string]interface{}); !ok {
			result.Errors = append(result.Errors, "'env' field must be a map/object")
		}
	}

	// Validate request section (required for HTTP operations)
	if request, exists := up["request"]; exists {
		requestMap, ok := request.(map[string]interface{})
		if !ok {
			result.Errors = append(result.Errors, "'request' section must be a map/object")
		} else {
			validateRequestSection(requestMap, result, "up")
		}
	}

	// Validate response section (optional but recommended)
	if response, exists := up["response"]; exists {
		responseMap, ok := response.(map[string]interface{})
		if !ok {
			result.Errors = append(result.Errors, "'response' section must be a map/object")
		} else {
			validateResponseSection(responseMap, result, "up")
		}
	} else {
		result.Warnings = append(result.Warnings, "No 'response' validation found in 'up' section - consider adding for better error handling")
	}

	// Validate find section (optional)
	if find, exists := up["find"]; exists {
		findMap, ok := find.(map[string]interface{})
		if !ok {
			result.Errors = append(result.Errors, "'find' section must be a map/object")
		} else {
			validateFindSection(findMap, result)
		}
	}
}

// validateDownSection validates the 'down' section of a migration
func validateDownSection(down map[string]interface{}, result *ValidationResult) {
	// Down section has similar structure to up but is optional
	// Check for required fields if down section exists
	requiredFields := []string{"name"}
	for _, field := range requiredFields {
		if _, exists := down[field]; !exists {
			result.Errors = append(result.Errors, fmt.Sprintf("Missing required field in 'down' section: '%s'", field))
		}
	}

	// Validate name field
	if name, exists := down["name"]; exists {
		if nameStr, ok := name.(string); !ok {
			result.Errors = append(result.Errors, "'name' field in 'down' section must be a string")
		} else if strings.TrimSpace(nameStr) == "" {
			result.Errors = append(result.Errors, "'name' field in 'down' section cannot be empty")
		}
	}

	// Validate env field (optional)
	if env, exists := down["env"]; exists {
		if _, ok := env.(map[string]interface{}); !ok {
			result.Errors = append(result.Errors, "'env' field in 'down' section must be a map/object")
		}
	}

	// Validate request section
	if request, exists := down["request"]; exists {
		requestMap, ok := request.(map[string]interface{})
		if !ok {
			result.Errors = append(result.Errors, "'request' section in 'down' must be a map/object")
		} else {
			validateRequestSection(requestMap, result, "down")
		}
	}

	// Validate response section
	if response, exists := down["response"]; exists {
		responseMap, ok := response.(map[string]interface{})
		if !ok {
			result.Errors = append(result.Errors, "'response' section in 'down' must be a map/object")
		} else {
			validateResponseSection(responseMap, result, "down")
		}
	}

	// Validate find section (optional)
	if find, exists := down["find"]; exists {
		findMap, ok := find.(map[string]interface{})
		if !ok {
			result.Errors = append(result.Errors, "'find' section in 'down' must be a map/object")
		} else {
			validateFindSection(findMap, result)
		}
	}
}

// validateRequestSection validates HTTP request configuration
func validateRequestSection(request map[string]interface{}, result *ValidationResult, prefix string) {
	// Check for required fields
	requiredFields := []string{"method", "url"}
	for _, field := range requiredFields {
		if _, exists := request[field]; !exists {
			result.Errors = append(result.Errors, fmt.Sprintf("Missing required field in '%s.request': '%s'", prefix, field))
		}
	}

	// Validate HTTP method
	if method, exists := request["method"]; exists {
		if methodStr, ok := method.(string); !ok {
			result.Errors = append(result.Errors, fmt.Sprintf("'%s.request.method' must be a string", prefix))
		} else {
			validMethods := map[string]bool{
				"GET": true, "POST": true, "PUT": true, "DELETE": true,
				"PATCH": true, "HEAD": true, "OPTIONS": true,
			}
			if !validMethods[strings.ToUpper(methodStr)] {
				result.Warnings = append(result.Warnings, fmt.Sprintf("'%s.request.method' uses non-standard HTTP method: '%s'", prefix, methodStr))
			}
		}
	}

	// Validate URL
	if url, exists := request["url"]; exists {
		if urlStr, ok := url.(string); !ok {
			result.Errors = append(result.Errors, fmt.Sprintf("'%s.request.url' must be a string", prefix))
		} else if strings.TrimSpace(urlStr) == "" {
			result.Errors = append(result.Errors, fmt.Sprintf("'%s.request.url' cannot be empty", prefix))
		}
	}

	// Validate headers (optional)
	if headers, exists := request["headers"]; exists {
		if _, ok := headers.(map[string]interface{}); !ok {
			result.Errors = append(result.Errors, fmt.Sprintf("'%s.request.headers' must be a map/object", prefix))
		}
	}

	// Validate body (optional)
	if body, exists := request["body"]; exists {
		// Body can be string or object
		switch body.(type) {
		case string, map[string]interface{}:
			// Valid types
		default:
			result.Warnings = append(result.Warnings, fmt.Sprintf("'%s.request.body' should be a string or object", prefix))
		}
	}
}

// validateResponseSection validates HTTP response validation configuration
func validateResponseSection(response map[string]interface{}, result *ValidationResult, prefix string) {
	// Check for result_code (most common validation)
	if resultCode, exists := response["result_code"]; exists {
		switch rc := resultCode.(type) {
		case []interface{}:
			// Array of status codes
			for i, code := range rc {
				switch code.(type) {
				case string, int:
					// Valid types
				default:
					result.Errors = append(result.Errors, fmt.Sprintf("'%s.response.result_code[%d]' must be a string or integer", prefix, i))
				}
			}
		case string, int:
			// Single status code
		default:
			result.Errors = append(result.Errors, fmt.Sprintf("'%s.response.result_code' must be a string, integer, or array", prefix))
		}
	}

	// Validate other response validation fields
	optionalFields := []string{"body_contains", "header_contains", "json_path"}
	for _, field := range optionalFields {
		if _, exists := response[field]; exists {
			// These fields exist - basic structure validation could be added here
			_ = exists // Mark as used to avoid static analysis warning
		}
	}
}

// validateFindSection validates the 'find' section for data extraction
func validateFindSection(find map[string]interface{}, result *ValidationResult) {
	// Check if at least one find method is specified
	findMethods := []string{"json_path", "regex", "xpath", "header"}
	hasMethod := false
	for _, method := range findMethods {
		if _, exists := find[method]; exists {
			hasMethod = true
			break
		}
	}

	if !hasMethod {
		result.Warnings = append(result.Warnings, "No extraction method specified in 'find' section (json_path, regex, xpath, header)")
	}
}
