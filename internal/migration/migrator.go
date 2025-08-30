package migration

import (
	"io"
	"os"
	"path/filepath"

	"github.com/loykin/apimigrate/internal/task"
	"gopkg.in/yaml.v3"
)

func loadTaskFromFile(path string) (task.Task, error) {
	clean := filepath.Clean(path)
	// #nosec G304 -- path comes from controlled directory listing of migration files
	f, err := os.Open(clean)
	if err != nil {
		return task.Task{}, err
	}
	defer func() { _ = f.Close() }()
	return decodeTaskYAML(f)
}

func decodeTaskYAML(r io.Reader) (task.Task, error) {
	dec := yaml.NewDecoder(r)
	var t task.Task
	if err := dec.Decode(&t); err != nil {
		return task.Task{}, err
	}
	return t, nil
}
