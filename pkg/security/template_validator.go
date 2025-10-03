package security

import (
	"fmt"
	"regexp"
	"strings"
	"text/template/parse"
)

var (
	ErrDangerousAction = fmt.Errorf("template contains dangerous action")
	ErrExcessiveDepth  = fmt.Errorf("template depth exceeds maximum allowed")
)

// TemplateValidator provides security validation for Go templates
type TemplateValidator struct {
	// MaxDepth limits template nesting depth to prevent stack overflow
	MaxDepth int
	// AllowedFunctions whitelist of allowed template functions
	AllowedFunctions map[string]bool
	// ForbiddenPatterns regex patterns that are not allowed in templates
	ForbiddenPatterns []*regexp.Regexp
}

// NewTemplateValidator creates a new validator with secure defaults
func NewTemplateValidator() *TemplateValidator {
	// Default allowed functions - only safe built-in functions
	allowedFunctions := map[string]bool{
		// String functions
		"printf":  true,
		"print":   true,
		"println": true,
		"len":     true,
		"index":   true,
		"slice":   true,
		"eq":      true,
		"ne":      true,
		"lt":      true,
		"le":      true,
		"gt":      true,
		"ge":      true,
		"and":     true,
		"or":      true,
		"not":     true,
		// Safe string manipulation
		"lower":     true,
		"upper":     true,
		"title":     true,
		"trim":      true,
		"trimLeft":  true,
		"trimRight": true,
		"contains":  true,
		"hasPrefix": true,
		"hasSuffix": true,
	}

	// Patterns that indicate potential security issues
	forbiddenPatterns := []*regexp.Regexp{
		// Prevent access to dangerous methods or fields
		regexp.MustCompile(`\.\s*[Ee]xec\s*\(`),   // .Exec( calls
		regexp.MustCompile(`\.\s*[Cc]md\s*\(`),    // .Cmd( calls
		regexp.MustCompile(`\.\s*[Ss]ystem\s*\(`), // .System( calls
		regexp.MustCompile(`\.\s*[Rr]un\s*\(`),    // .Run( calls
		regexp.MustCompile(`\.\s*[Ss]tart\s*\(`),  // .Start( calls
		regexp.MustCompile(`\.\s*[Oo]pen\s*\(`),   // .Open( calls
		regexp.MustCompile(`\.\s*[Cc]reate\s*\(`), // .Create( calls
		regexp.MustCompile(`\.\s*[Ww]rite\s*\(`),  // .Write( calls
		regexp.MustCompile(`\.\s*[Dd]elete\s*\(`), // .Delete( calls
		regexp.MustCompile(`\.\s*[Rr]emove\s*\(`), // .Remove( calls
		// Prevent path traversal attempts
		regexp.MustCompile(`\.\./`),      // ../ path traversal
		regexp.MustCompile(`\\\.\\\.\\`), // ..\ path traversal
		// Prevent access to environment variables unsafely (shell expansion)
		// This pattern specifically targets ${var} syntax but excludes Go templates like ${{.env.name}}
		regexp.MustCompile(`\$\{[^{][^}]*}`), // ${var} shell expansion (not ${{template}})
		regexp.MustCompile("`[^`]*`"),        // Backtick execution
	}

	return &TemplateValidator{
		MaxDepth:          10, // Reasonable depth limit
		AllowedFunctions:  allowedFunctions,
		ForbiddenPatterns: forbiddenPatterns,
	}
}

// ValidateTemplate validates a template string for security issues
func (v *TemplateValidator) ValidateTemplate(templateStr string) error {
	if templateStr == "" {
		return nil
	}

	// Check for forbidden patterns first (fast check)
	if err := v.checkForbiddenPatterns(templateStr); err != nil {
		return err
	}

	// Parse the template to analyze its structure
	tree, err := parse.Parse("validator", templateStr, "{{", "}}")
	if err != nil {
		// If we can't parse it, it's probably safe from injection but invalid
		return fmt.Errorf("template parse error: %w", err)
	}

	// Validate the parsed template tree
	for _, node := range tree["validator"].Root.Nodes {
		if err := v.validateNode(node, 0); err != nil {
			return err
		}
	}

	return nil
}

// checkForbiddenPatterns checks for dangerous patterns in the template
func (v *TemplateValidator) checkForbiddenPatterns(templateStr string) error {
	for _, pattern := range v.ForbiddenPatterns {
		if pattern.MatchString(templateStr) {
			return fmt.Errorf("%w: pattern '%s' matched forbidden regex", ErrDangerousAction, pattern.String())
		}
	}
	return nil
}

// validateNode recursively validates a template node
func (v *TemplateValidator) validateNode(node parse.Node, depth int) error {
	if depth > v.MaxDepth {
		return ErrExcessiveDepth
	}

	switch n := node.(type) {
	case *parse.ActionNode:
		return v.validateAction(n, depth)
	case *parse.IfNode:
		return v.validateIfNode(n, depth)
	case *parse.RangeNode:
		return v.validateRangeNode(n, depth)
	case *parse.WithNode:
		return v.validateWithNode(n, depth)
	case *parse.TemplateNode:
		return v.validateTemplateNode(n, depth)
	case *parse.ListNode:
		return v.validateListNode(n, depth)
	}
	return nil
}

// validateAction validates an action node ({{ ... }})
func (v *TemplateValidator) validateAction(node *parse.ActionNode, depth int) error {
	for _, cmd := range node.Pipe.Cmds {
		for _, arg := range cmd.Args {
			if err := v.validateArg(arg, depth); err != nil {
				return err
			}
		}
	}
	return nil
}

// validateArg validates template arguments
func (v *TemplateValidator) validateArg(arg parse.Node, depth int) error {
	switch a := arg.(type) {
	case *parse.IdentifierNode:
		// For template parsing, we only validate dangerous patterns
		// The actual function validation will be done by the template engine
		return v.validateIdentifier(a.Ident)
	case *parse.FieldNode:
		// Validate field access
		return v.validateFieldAccess(a.Ident)
	case *parse.ChainNode:
		// Validate chained operations
		return v.validateChain(a, depth)
	}
	return nil
}

// validateIdentifier checks if an identifier contains dangerous patterns
func (v *TemplateValidator) validateIdentifier(ident string) error {
	lower := strings.ToLower(ident)
	// Check for dangerous function patterns
	if strings.Contains(lower, "exec") ||
		strings.Contains(lower, "system") ||
		strings.Contains(lower, "cmd") ||
		strings.Contains(lower, "eval") ||
		strings.Contains(lower, "run") {
		return fmt.Errorf("%w: dangerous identifier '%s'", ErrDangerousAction, ident)
	}
	return nil
}

// validateFieldAccess validates field access for dangerous patterns
func (v *TemplateValidator) validateFieldAccess(fields []string) error {
	for _, field := range fields {
		lower := strings.ToLower(field)
		// Block access to potentially dangerous fields
		if strings.Contains(lower, "exec") ||
			strings.Contains(lower, "cmd") ||
			strings.Contains(lower, "system") ||
			strings.Contains(lower, "run") ||
			strings.Contains(lower, "eval") {
			return fmt.Errorf("%w: dangerous field access '%s'", ErrDangerousAction, field)
		}
	}
	return nil
}

// validateChain validates chained method calls
func (v *TemplateValidator) validateChain(chain *parse.ChainNode, _ int) error {
	for _, field := range chain.Field {
		if err := v.validateFieldAccess([]string{field}); err != nil {
			return err
		}
	}
	return nil
}

// Helper methods for different node types
func (v *TemplateValidator) validateIfNode(node *parse.IfNode, depth int) error {
	if err := v.validateListNode(node.List, depth+1); err != nil {
		return err
	}
	if node.ElseList != nil {
		return v.validateListNode(node.ElseList, depth+1)
	}
	return nil
}

func (v *TemplateValidator) validateRangeNode(node *parse.RangeNode, depth int) error {
	if err := v.validateListNode(node.List, depth+1); err != nil {
		return err
	}
	if node.ElseList != nil {
		return v.validateListNode(node.ElseList, depth+1)
	}
	return nil
}

func (v *TemplateValidator) validateWithNode(node *parse.WithNode, depth int) error {
	if err := v.validateListNode(node.List, depth+1); err != nil {
		return err
	}
	if node.ElseList != nil {
		return v.validateListNode(node.ElseList, depth+1)
	}
	return nil
}

func (v *TemplateValidator) validateTemplateNode(node *parse.TemplateNode, _ int) error {
	// Template inclusion - validate the name isn't dangerous
	if strings.Contains(node.Name, "..") || strings.Contains(node.Name, "/") {
		return fmt.Errorf("%w: dangerous template name '%s'", ErrDangerousAction, node.Name)
	}
	return nil
}

func (v *TemplateValidator) validateListNode(node *parse.ListNode, depth int) error {
	if node == nil {
		return nil
	}
	for _, child := range node.Nodes {
		if err := v.validateNode(child, depth); err != nil {
			return err
		}
	}
	return nil
}

// SanitizeInput sanitizes user input to prevent basic injection attacks
func (v *TemplateValidator) SanitizeInput(input string) string {
	// Remove potential dangerous characters
	sanitized := input

	// Remove backticks completely (including content)
	re := regexp.MustCompile("`[^`]*`")
	sanitized = re.ReplaceAllString(sanitized, "")

	// Remove remaining backticks
	sanitized = strings.ReplaceAll(sanitized, "`", "")
	sanitized = strings.ReplaceAll(sanitized, "$", "")
	sanitized = strings.ReplaceAll(sanitized, "|", "")
	sanitized = strings.ReplaceAll(sanitized, ";", "")

	// HTML escape
	sanitized = strings.ReplaceAll(sanitized, "&", "&amp;")
	sanitized = strings.ReplaceAll(sanitized, "<", "&lt;")
	sanitized = strings.ReplaceAll(sanitized, ">", "&gt;")

	return sanitized
}
