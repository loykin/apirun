package migration

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"

	"github.com/loykin/apimigrate/internal/task"
)

func planUp(files []vfile, cur, target int) []vfile {
	limit := target
	if limit <= 0 {
		limit = 1<<31 - 1
	}
	plan := make([]vfile, 0)
	for _, f := range files {
		if f.index > cur && f.index <= limit {
			plan = append(plan, f)
		}
	}
	sort.Slice(plan, func(i, j int) bool { return plan[i].index < plan[j].index })
	return plan
}

func mapFilesByVersion(files []vfile) map[int]vfile {
	m := map[int]vfile{}
	for _, f := range files {
		m[f.index] = f
	}
	return m
}

var versionFileRegex = regexp.MustCompile(`^(\d+)_.*\.(ya?ml)$`)

type vfile struct {
	index int
	name  string
	path  string
}

func listMigrationFiles(dir string) ([]vfile, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var files []vfile
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		m := versionFileRegex.FindStringSubmatch(name)
		if len(m) == 0 {
			continue
		}
		var idx int
		_, err := fmt.Sscanf(m[1], "%d", &idx)
		if err != nil {
			continue
		}
		files = append(files, vfile{index: idx, name: name, path: filepath.Join(dir, name)})
	}
	sort.Slice(files, func(i, j int) bool { return files[i].index < files[j].index })
	return files, nil
}

// ExecWithVersion pairs ExecResult with version number.
type ExecWithVersion struct {
	Version int
	Result  *task.ExecResult
}
