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

	"github.com/loykin/apimigrate/pkg/env"
	"github.com/loykin/apimigrate/pkg/task"
	"gopkg.in/yaml.v3"
)

// migrationFileRegex matches files like 001_init.yaml, 10_feature.yml, etc.
var migrationFileRegex = regexp.MustCompile(`^(\d+)_.*\.(ya?ml)$`)

// RunMigrationsWithEnv scans the provided directory for migration YAML files (goose-like naming),
// sorts them by their numeric prefix, loads each into a Task, and executes Up sequentially.
// It stops on the first error and returns the results collected so far and the error.
// baseEnv, if provided, is used to populate Global env for each task execution.
func RunMigrationsWithEnv(ctx context.Context, dir, method, url string, baseEnv env.Env) ([]*task.ExecResult, error) {
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
		// Initialize layered env for this task: Global from base, Local from task YAML
		if baseEnv.Global != nil {
			t.Up.Env.Global = baseEnv.Global
		} else {
			t.Up.Env.Global = map[string]string{}
		}
		// Local is already decoded from YAML into t.Up.Env.Local; ensure non-nil
		if t.Up.Env.Local == nil {
			t.Up.Env.Local = map[string]string{}
		}

		res, err := t.UpExecute(ctx, method, url)
		results = append(results, res)
		if err != nil {
			return results, fmt.Errorf("migration %s failed: %w", f.name, err)
		}
	}
	return results, nil
}

// RunMigrations is kept for backward compatibility; it runs without injecting any base env.
func RunMigrations(ctx context.Context, dir, method, url string) ([]*task.ExecResult, error) {
	return RunMigrationsWithEnv(ctx, dir, method, url, env.Env{Global: map[string]string{}})
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
