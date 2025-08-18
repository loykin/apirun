package migration

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"

	"github.com/loykin/apimigrate/pkg/task"
	"gopkg.in/yaml.v3"
)

// migrationFileRegex matches files like 001_init.yaml, 10_feature.yml, etc.
var migrationFileRegex = regexp.MustCompile(`^(\d+)_.*\.(ya?ml)$`)

// RunMigrations scans the provided directory for migration YAML files (goose-like naming),
// sorts them by their numeric prefix, loads each into a Task, and executes Up sequentially.
// It stops on the first error and returns the results collected so far and the error.
func RunMigrations(ctx context.Context, dir, method, url string) ([]*task.ExecResult, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	type mf struct {
		index int
		name  string
		path  string
	}
	var files []mf
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		m := migrationFileRegex.FindStringSubmatch(name)
		if len(m) == 0 {
			continue
		}
		var idx int
		_, err := fmt.Sscanf(m[1], "%d", &idx)
		if err != nil {
			continue
		}
		files = append(files, mf{index: idx, name: name, path: filepath.Join(dir, name)})
	}

	if len(files) == 0 {
		return nil, errors.New("no migration files found")
	}

	sort.Slice(files, func(i, j int) bool { return files[i].index < files[j].index })

	results := make([]*task.ExecResult, 0, len(files))
	for _, f := range files {
		t, err := loadTaskFromFile(f.path)
		if err != nil {
			return results, fmt.Errorf("failed to load %s: %w", f.name, err)
		}
		res, err := t.UpExecute(ctx, method, url)
		results = append(results, res)
		if err != nil {
			return results, fmt.Errorf("migration %s failed: %w", f.name, err)
		}
	}
	return results, nil
}

func loadTaskFromFile(path string) (task.Task, error) {
	f, err := os.Open(path)
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
