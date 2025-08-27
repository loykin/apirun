package task

// Header represents a single header key-value pair.
type Header struct {
	Name  string `yaml:"name"`
	Value string `yaml:"value"`
}

// Query represents a single query parameter key-value pair.
type Query struct {
	Name  string `yaml:"name"`
	Value string `yaml:"value"`
}

// ExecResult contains the outcome of an execution of Up.
type ExecResult struct {
	StatusCode int
	// Extracted environment variables as per EnvFrom mapping.
	ExtractedEnv map[string]string
	// Raw response body as a string; may be empty on network error.
	ResponseBody string
}
