package task

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type Task struct {
	Up   Up   `yaml:"up"`
	Down Down `yaml:"down"`
}

// decodeYAMLTo is an internal helper to unmarshal YAML into the provided Task.
func (t *Task) decodeYAMLTo(r io.Reader) error {
	dec := yaml.NewDecoder(r)
	var tmp Task
	if err := dec.Decode(&tmp); err != nil {
		return fmt.Errorf("failed to decode YAML task configuration: %w", err)
	}
	*t = tmp
	return nil
}

// LoadFromFile loads a Task from a YAML file path into the receiver.
func (t *Task) LoadFromFile(path string) error {
	clean := filepath.Clean(path)
	// #nosec G304 -- path is provided by controlled migration listing
	f, err := os.Open(clean)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	return t.decodeYAMLTo(f)
}

// DecodeYAML decodes a Task from the provided reader into the receiver.
// Exposed for tests in other packages if needed.
func (t *Task) DecodeYAML(r io.Reader) error { //nolint:unused // may be used by external tests
	return t.decodeYAMLTo(r)
}
