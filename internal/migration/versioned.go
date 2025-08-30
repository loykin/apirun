package migration

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"

	"github.com/loykin/apimigrate/internal/env"
	"github.com/loykin/apimigrate/internal/store"
	"github.com/loykin/apimigrate/internal/task"
)

func saveResponseBodies(ctx context.Context) bool {
	save := false
	if v := ctx.Value(SaveResponseBodyKey); v != nil {
		if b, ok := v.(bool); ok {
			save = b
		}
	}
	return save
}

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

// MigrateUp applies migrations greater than the current store version up to targetVersion.
// If targetVersion <= 0, it applies all pending migrations.
// It records each applied version in the store after successful execution.
func runUpForFile(ctx context.Context, st *store.Store, f vfile, baseEnv env.Env, sessionStored map[string]string) (*ExecWithVersion, map[string]string, error) {
	t, err := loadTaskFromFile(f.path)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to load %s: %w", f.name, err)
	}
	if baseEnv.Global != nil {
		t.Up.Env.Global = baseEnv.Global
	} else {
		t.Up.Env.Global = map[string]string{}
	}
	if t.Up.Env.Local == nil {
		t.Up.Env.Local = map[string]string{}
	}
	// Merge stored env from previously applied versions
	if applied, err := st.ListApplied(); err == nil {
		for _, av := range applied {
			if m, _ := st.LoadStoredEnv(av); len(m) > 0 {
				for k, val := range m {
					if _, exists := t.Up.Env.Local[k]; !exists {
						t.Up.Env.Local[k] = val
					}
				}
			}
		}
	}
	// Merge sessionStored for this run
	if len(sessionStored) > 0 {
		for k, val := range sessionStored {
			if _, exists := t.Up.Env.Local[k]; !exists {
				t.Up.Env.Local[k] = val
			}
		}
	}
	res, err := t.UpExecute(ctx, "", "")
	ewv := &ExecWithVersion{Version: f.index, Result: res}
	// Record run if we have result
	if res != nil {
		save := saveResponseBodies(ctx)
		var bodyPtr *string
		if save {
			b := res.ResponseBody
			bodyPtr = &b
		}
		// persist extracted env
		toStore := map[string]string{}
		if res.ExtractedEnv != nil {
			toStore = res.ExtractedEnv
		}
		_ = st.RecordRun(f.index, "up", res.StatusCode, bodyPtr, toStore)
		_ = st.InsertStoredEnv(f.index, toStore)
		return ewv, toStore, err
	}
	return ewv, nil, err
}

// MigrateDown rolls back down to targetVersion (not including target): it will
// run downs for all applied versions > targetVersion in reverse order.
// Each successful down removes that version from the store.
func runDownForVersion(ctx context.Context, st *store.Store, ver int, f vfile, baseEnv env.Env) (*ExecWithVersion, error) {
	t, err := loadTaskFromFile(f.path)
	if err != nil {
		return nil, fmt.Errorf("failed to load %s: %w", f.name, err)
	}
	if baseEnv.Global != nil {
		t.Down.Env.Global = baseEnv.Global
	} else {
		t.Down.Env.Global = map[string]string{}
	}
	if t.Down.Env.Local == nil {
		t.Down.Env.Local = map[string]string{}
	}
	// Merge stored env for this version (prefer stored_env; fallback to legacy)
	if loaded, _ := st.LoadStoredEnv(ver); len(loaded) > 0 {
		for k, val := range loaded {
			if _, exists := t.Down.Env.Local[k]; !exists {
				t.Down.Env.Local[k] = val
			}
		}
	} else if loadedLegacy, _ := st.LoadEnv(ver, "up"); len(loadedLegacy) > 0 {
		for k, val := range loadedLegacy {
			if _, exists := t.Down.Env.Local[k]; !exists {
				t.Down.Env.Local[k] = val
			}
		}
	}
	res, err := t.DownExecute(ctx, "", "")
	ewv := &ExecWithVersion{Version: ver, Result: res}
	if res != nil {
		save := saveResponseBodies(ctx)
		var bodyPtr *string
		if save {
			b := res.ResponseBody
			bodyPtr = &b
		}
		_ = st.RecordRun(ver, "down", res.StatusCode, bodyPtr, nil)
	}
	if err != nil {
		return ewv, fmt.Errorf("down %s failed: %w", f.name, err)
	}
	if err := st.Remove(ver); err != nil {
		return ewv, fmt.Errorf("record remove %d: %w", ver, err)
	}
	_ = st.DeleteStoredEnv(ver)
	return ewv, nil
}

// ExecWithVersion pairs ExecResult with version number.
type ExecWithVersion struct {
	Version int
	Result  *task.ExecResult
}
