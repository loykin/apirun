# Sensitive Data Masking Demo

This example demonstrates the sensitive data masking capabilities of apirun, which automatically masks sensitive
information in logs to enhance security.

## Overview

The masking system automatically detects and masks sensitive information such as:

- Passwords
- API keys
- Tokens (JWT, Bearer, etc.)
- Authorization headers
- Client secrets

## Features Demonstrated

### 1. Automatic Masking

```go
logger.Info("user authentication",
"username", "admin",
"password", "secret123",         // → ***MASKED***
"api_key", "sk_test_1234567890") // → ***MASKED***
```

### 2. Authorization Header Masking

```go
logger.Info("HTTP request",
"authorization", "Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9") // → ***MASKED***
```

### 3. Dynamic Masking Control

```go
logger.EnableMasking(false) // Disable masking (for development)
logger.EnableMasking(true) // Re-enable masking (for production)
```

### 4. Global Masking Functions

```go
sensitiveText := `{"password": "secret123", "api_key": "sk_test_1234"}`
masked := apirun.MaskSensitiveData(sensitiveText)
// Result: {"password":"***MASKED***", "api_key":"***MASKED***"}
```

### 5. Custom Masking Patterns

```go
masker := apirun.NewMasker()
customPattern := apirun.SensitivePattern{
Name: "custom_token",
Keys: []string{"custom_token", "my_secret"},
}
masker.AddPattern(customPattern)
logger.SetMasker(masker)
```

## Running the Demo

```bash
go run ./examples/masking_demo
```

## Expected Output

```
=== Testing Sensitive Data Masking ===
time=2025-09-18T01:16:15.248+09:00 level=INFO msg="user authentication" username=admin password=***MASKED*** api_key=***MASKED***
time=2025-09-18T01:16:15.248+09:00 level=INFO msg="HTTP request" method=POST url=/api/users authorization=***MASKED***
time=2025-09-18T01:16:15.248+09:00 level=INFO msg="user data" username=john.doe email=john@example.com role=admin
time=2025-09-18T01:16:15.248+09:00 level=INFO msg="masking disabled - sensitive data visible" password=this_will_be_visible token=visible_token
time=2025-09-18T01:16:15.248+09:00 level=INFO msg="masking re-enabled - sensitive data hidden" password=***MASKED*** client_secret=***MASKED***

Original: {"username": "admin", "password": "secret123", "api_key": "sk_test_1234"}
Masked:   {"username": "admin", "password":"***MASKED***", "api_key":"***MASKED***"}

time=2025-09-18T01:16:15.248+09:00 level=INFO msg="custom pattern test" custom_token=***MASKED*** my_secret=***MASKED*** normal_field=visible_data

=== Masking Demo Complete ===
```

## Configuration

You can configure masking in your `config.yaml`:

```yaml
logging:
  level: info
  format: text  # or "json"
  mask_sensitive: true  # Enable/disable sensitive data masking
```

## Supported Patterns

The masking system includes built-in patterns for:

| Pattern Type      | Keys                                  | Example Values                            |
|-------------------|---------------------------------------|-------------------------------------------|
| **Password**      | `password`, `passwd`, `pwd`           | `secret123`                               |
| **API Key**       | `api_key`, `apikey`, `api-key`        | `sk_test_1234567890abcdef`                |
| **Token**         | `token`, `access_token`, `auth_token` | `eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...` |
| **Authorization** | `authorization`                       | `Bearer abc123`, `Basic dXNlcjpwYXNz`     |
| **Client Secret** | `client_secret`, `secret`             | `client_secret_abc123`                    |

## Security Benefits

1. **Prevents credential leakage** in log files
2. **Configurable** - can be disabled for development
3. **Extensible** - custom patterns can be added
4. **Performance optimized** - zero overhead when disabled
5. **Regex-based** - flexible pattern matching

## Integration with API Migration

When running API migrations, sensitive authentication data is automatically masked:

```go
migrator.Log().Info("Starting migration",
"source_api_key", "sk-source-secret123", // → ***MASKED***
"target_api_key", "sk-target-secret456", // → ***MASKED***
"auth_token", "Bearer migration-token-xyz") // → ***MASKED***
```

This ensures that migration logs are secure and can be safely stored or transmitted without exposing sensitive
credentials.
