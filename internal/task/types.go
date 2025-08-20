package task

// Header represents a single header key-value pair.
type Header struct {
	Name  string `yaml:"name" json:"name"`
	Value string `yaml:"value" json:"value"`
}

// Query represents a single query parameter key-value pair.
type Query struct {
	Name  string `yaml:"name" json:"name"`
	Value string `yaml:"value" json:"value"`
}

// ExecResult contains the outcome of an execution of Up.
type ExecResult struct {
	StatusCode int
	// Extracted environment variables as per EnvFrom mapping.
	ExtractedEnv map[string]string
}
