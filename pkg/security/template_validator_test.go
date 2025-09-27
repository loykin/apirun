package security

import (
	"strings"
	"testing"
)

func TestTemplateValidator_ValidateTemplate(t *testing.T) {
	validator := NewTemplateValidator()

	tests := []struct {
		name        string
		template    string
		expectError bool
		errorType   error
	}{
		{
			name:        "valid simple template",
			template:    "Hello {{.username}}",
			expectError: false,
		},
		{
			name:        "valid complex template",
			template:    "{{if .username}}Hello {{.username}}{{else}}Hello guest{{end}}",
			expectError: false,
		},
		{
			name:        "valid field access",
			template:    "{{.env.username}}",
			expectError: false,
		},
		{
			name:        "dangerous exec pattern",
			template:    "{{.env.Exec(\"rm -rf /\")}}",
			expectError: true,
			errorType:   ErrDangerousAction,
		},
		{
			name:        "dangerous cmd pattern",
			template:    "{{.system.Cmd(\"evil\")}}",
			expectError: true,
			errorType:   ErrDangerousAction,
		},
		{
			name:        "path traversal attempt",
			template:    "{{.file \"../../../etc/passwd\"}}",
			expectError: true,
			errorType:   ErrDangerousAction,
		},
		{
			name:        "shell expansion attempt",
			template:    "Hello ${USER}",
			expectError: true,
			errorType:   ErrDangerousAction,
		},
		{
			name:        "backtick execution attempt",
			template:    "Hello `whoami`",
			expectError: true,
			errorType:   ErrDangerousAction,
		},
		{
			name:        "dangerous identifier",
			template:    "{{.eval_code}}",
			expectError: true,
			errorType:   ErrDangerousAction,
		},
		{
			name:        "dangerous field access",
			template:    "{{.runtime.Exec}}",
			expectError: true,
			errorType:   ErrDangerousAction,
		},
		{
			name:        "empty template",
			template:    "",
			expectError: false,
		},
		{
			name:        "valid nesting within limits",
			template:    "{{if .level1}}{{if .level2}}content{{end}}{{end}}",
			expectError: false,
		},
		{
			name:        "valid template with dollar prefix",
			template:    "${{.env.name}}",
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.ValidateTemplate(tt.template)

			if tt.expectError {
				if err == nil {
					t.Errorf("expected error but got none for template: %s", tt.template)
					return
				}

				if tt.errorType != nil {
					if !strings.Contains(err.Error(), tt.errorType.Error()) {
						t.Errorf("expected error type %v, got %v", tt.errorType, err)
					}
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v for template: %s", err, tt.template)
				}
			}
		})
	}
}

func TestTemplateValidator_SanitizeInput(t *testing.T) {
	validator := NewTemplateValidator()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "clean input",
			input:    "hello world",
			expected: "hello world",
		},
		{
			name:     "remove backticks",
			input:    "hello `whoami` world",
			expected: "hello  world",
		},
		{
			name:     "remove dollar signs",
			input:    "hello $USER world",
			expected: "hello USER world",
		},
		{
			name:     "escape HTML",
			input:    "hello <script>alert('xss')</script>",
			expected: "hello &lt;script&gt;alert('xss')&lt;/script&gt;",
		},
		{
			name:     "remove dangerous chars",
			input:    "cmd | grep; rm",
			expected: "cmd  grep rm",
		},
		{
			name:     "escape ampersand",
			input:    "hello & world",
			expected: "hello &amp; world",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := validator.SanitizeInput(tt.input)
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestTemplateValidator_CustomConfiguration(t *testing.T) {
	// Test with custom validator configuration
	validator := NewTemplateValidator()
	validator.MaxDepth = 20 // Allow deeper nesting

	template := "{{if .level1}}{{if .level2}}content{{end}}{{end}}"
	err := validator.ValidateTemplate(template)
	if err != nil {
		t.Errorf("expected template to be valid with custom config, got error: %v", err)
	}
}

func TestTemplateValidator_MaxDepthConfiguration(t *testing.T) {
	validator := NewTemplateValidator()
	validator.MaxDepth = 2

	// This should fail with the lower depth limit
	template := generateNestedTemplate(5)
	err := validator.ValidateTemplate(template)
	if err == nil {
		t.Error("expected error for excessive depth with custom limit")
	}
}

// generateNestedTemplate creates a template with specified nesting depth
func generateNestedTemplate(depth int) string {
	template := "{{if .level0}}"
	for i := 1; i < depth; i++ {
		template += strings.Repeat(" ", i) + "{{if .level" + string(rune('0'+i)) + "}}"
	}
	template += "content"
	for i := depth - 1; i >= 0; i-- {
		template += "{{end}}"
	}
	return template
}

func BenchmarkValidateTemplate(b *testing.B) {
	validator := NewTemplateValidator()
	template := "{{if .username}}Hello {{printf \"%s\" .username}}{{else}}Hello guest{{end}}"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = validator.ValidateTemplate(template)
	}
}

func BenchmarkSanitizeInput(b *testing.B) {
	validator := NewTemplateValidator()
	input := "hello `whoami` $USER & <script>alert('xss')</script>"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = validator.SanitizeInput(input)
	}
}
