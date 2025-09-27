# Security Package

The `pkg/security` package provides comprehensive template security validation for the APIRun project, designed to prevent template injection attacks and other security vulnerabilities while maintaining legitimate template functionality.

## Overview

This package implements a robust template validation system that analyzes Go templates for potentially dangerous patterns before they are processed. It helps protect against:

- Template injection attacks
- Shell command execution attempts
- Path traversal attacks
- Unsafe function calls
- Environment variable exploitation

## Features

### Template Validation
- **Dangerous Function Detection**: Identifies and blocks calls to potentially harmful functions like `exec`, `cmd`, `system`, etc.
- **Path Traversal Prevention**: Prevents directory traversal attempts using patterns like `../` or `..\`
- **Shell Injection Protection**: Blocks shell command execution patterns including backticks, shell expansions, and command substitution
- **Template Depth Limiting**: Enforces maximum nesting depth to prevent resource exhaustion
- **Input Sanitization**: Provides HTML escaping and dangerous character removal

### Smart Pattern Recognition
- Distinguishes between legitimate Go templates (`${{.env.name}}`) and dangerous shell expansions (`${USER}`)
- Allows safe template operations while blocking potentially harmful ones
- Configurable allowed function whitelist

## Usage

### Basic Template Validation

```go
import "github.com/loykin/apirun/pkg/security"

// Create a validator with default settings
validator := security.NewTemplateValidator()

// Validate a template string
template := "Hello {{.env.name}}!"
err := validator.ValidateTemplate(template)
if err != nil {
    // Handle validation error
    log.Printf("Template validation failed: %v", err)
}
```

### Custom Validator Configuration

```go
// Create validator with custom settings
validator := &security.TemplateValidator{
    MaxDepth: 5, // Limit template nesting depth
    AllowedFunctions: map[string]bool{
        "printf": true,
        "html":   true,
        // Add custom allowed functions
    },
    ForbiddenPatterns: []*regexp.Regexp{
        regexp.MustCompile(`custom_dangerous_pattern`),
        // Add custom forbidden patterns
    },
}

err := validator.ValidateTemplate(templateString)
```

### Input Sanitization

```go
// Sanitize user input before template processing
userInput := "<script>alert('xss')</script>"
sanitized := validator.SanitizeInput(userInput)
// Result: "&lt;script&gt;alert('xss')&lt;/script&gt;"
```

## Security Patterns

### Blocked Patterns

The validator automatically blocks these dangerous patterns:

#### Function Calls
- `.Exec(`, `.exec(`
- `.Cmd(`, `.cmd(`
- `.System(`, `.system(`
- `.Run(`, `.run(`
- `.Start(`, `.start(`
- `.Open(`, `.open(`
- `.Create(`, `.create(`
- `.Write(`, `.write(`
- `.Delete(`, `.delete(`
- `.Remove(`, `.remove(`

#### Path Traversal
- `../` (Unix path traversal)
- `..\` (Windows path traversal)

#### Shell Injection
- `` `command` `` (Backtick execution)
- `${variable}` (Shell expansion - but allows `${{.template}}`)

#### Dangerous Identifiers
- Templates containing `eval_code`, `runtime`, or other suspicious identifiers

### Allowed Patterns

These legitimate template patterns are explicitly allowed:

- `{{.env.variable}}` - Standard Go template variable access
- `${{.env.name}}` - Go template with dollar prefix
- `{{if .condition}}...{{end}}` - Conditional templates
- `{{range .items}}...{{end}}` - Loop templates
- Basic arithmetic and string operations

## Error Types

The package defines specific error types for different validation failures:

```go
var (
    ErrDangerousAction  = errors.New("dangerous action detected")
    ErrMaxDepthExceeded = errors.New("maximum template depth exceeded")
    ErrInvalidSyntax    = errors.New("invalid template syntax")
)
```

## Integration

The security validator is automatically integrated into the APIRun template rendering pipeline:

```go
// In pkg/env/env.go
func (e *Env) RenderGoTemplateErr(s string) (string, error) {
    // Security validation applied automatically
    validator := security.NewTemplateValidator()
    if err := validator.ValidateTemplate(s); err != nil {
        return "", fmt.Errorf("template security validation failed: %w", err)
    }
    // ... proceed with template rendering
}
```

## Testing

The package includes comprehensive test coverage:

- **Valid Template Tests**: Ensures legitimate templates pass validation
- **Security Attack Tests**: Verifies dangerous patterns are blocked
- **Edge Case Tests**: Handles empty inputs, complex nesting, etc.
- **Custom Configuration Tests**: Validates configurable behavior

Run tests:
```bash
go test ./pkg/security -v
```

## Configuration Examples

### Permissive Configuration
```go
validator := &security.TemplateValidator{
    MaxDepth: 20,
    AllowedFunctions: map[string]bool{
        "printf":   true,
        "html":     true,
        "url":      true,
        "json":     true,
        "base64":   true,
    },
    // Minimal forbidden patterns for development
}
```

### Strict Configuration
```go
validator := &security.TemplateValidator{
    MaxDepth: 3,
    AllowedFunctions: map[string]bool{
        "html": true, // Only HTML escaping allowed
    },
    // Maximum security for production
}
```

## Best Practices

1. **Always Validate**: Never process user-provided templates without validation
2. **Use Default Settings**: The default validator configuration provides good security for most use cases
3. **Whitelist Functions**: Only allow specific template functions that your application needs
4. **Monitor Logs**: Log validation failures for security monitoring
5. **Regular Updates**: Keep security patterns updated as new threats emerge

## Performance Considerations

- Template validation adds minimal overhead (~1-2ms per template)
- Validation results can be cached for repeated templates
- Complex regex patterns may impact performance on very large templates
- Consider validation timeouts for user-facing applications

## Migration from Previous Versions

If upgrading from a version without security validation:

1. Test existing templates with the new validator
2. Update any templates that trigger false positives
3. Configure custom allowed functions if needed
4. Monitor logs for validation failures after deployment

## Contributing

When adding new security patterns:

1. Add comprehensive test cases
2. Consider performance impact
3. Document the security risk being prevented
4. Ensure legitimate use cases are not blocked