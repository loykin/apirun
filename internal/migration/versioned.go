package migration

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"time"

	"github.com/loykin/apimigrate/internal/env"
	"github.com/loykin/apimigrate/internal/store"
	"github.com/loykin/apimigrate/internal/task"
)

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
func MigrateUp(ctx context.Context, dir string, baseEnv env.Env, targetVersion int) ([]*ExecWithVersion, error) {
	files, err := listMigrationFiles(dir)
	if err != nil {
		return nil, err
	}
	// open store at default path
	dbPath := filepath.Join(dir, store.DbFileName)
	st, err := store.Open(dbPath)
	if err != nil {
		return nil, err
	}
	defer func() { _ = st.Close() }()

	cur, err := st.CurrentVersion()
	if err != nil {
		return nil, err
	}
	limit := targetVersion
	if limit <= 0 {
		limit = 1<<31 - 1
	}

	// plan versions to run
	plan := make([]vfile, 0)
	for _, f := range files {
		if f.index > cur && f.index <= limit {
			plan = append(plan, f)
		}
	}
	sort.Slice(plan, func(i, j int) bool { return plan[i].index < plan[j].index })

	results := make([]*ExecWithVersion, 0, len(plan))
	// sessionStored accumulates stored env created during this run to be available to later versions
	sessionStored := map[string]string{}
	for _, f := range plan {
		t, err := loadTaskFromFile(f.path)
		if err != nil {
			return results, fmt.Errorf("failed to load %s: %w", f.name, err)
		}
		// Set env layering per existing behavior
		if baseEnv.Global != nil {
			t.Up.Env.Global = baseEnv.Global
		} else {
			t.Up.Env.Global = map[string]string{}
		}
		if t.Up.Env.Local == nil {
			t.Up.Env.Local = map[string]string{}
		}
		// Merge stored env from previously applied versions and from earlier steps in this run
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
		if len(sessionStored) > 0 {
			for k, val := range sessionStored {
				if _, exists := t.Up.Env.Local[k]; !exists {
					t.Up.Env.Local[k] = val
				}
			}
		}
		res, err := t.UpExecute(ctx, "", "")
		results = append(results, &ExecWithVersion{Version: f.index, Result: res})
		// record run regardless of success if we have a result
		if res != nil {
			save := false
			if v := ctx.Value(SaveResponseBodyKey); v != nil {
				if b, ok := v.(bool); ok {
					save = b
				}
			}
			var bodyPtr *string
			if save {
				b := res.ResponseBody
				bodyPtr = &b
			}
			// persist all extracted env (auto-store) into run record and stored_env
			var toStore map[string]string
			if res.ExtractedEnv != nil {
				toStore = res.ExtractedEnv
			} else {
				toStore = map[string]string{}
			}
			_ = st.RecordRun(f.index, "up", res.StatusCode, bodyPtr, toStore)
			// also insert into stored_env for reuse and lifecycle mgmt
			_ = st.InsertStoredEnv(f.index, toStore)
			// update sessionStored so subsequent versions in the same run can use these values
			if len(toStore) > 0 {
				for k, v := range toStore {
					sessionStored[k] = v
				}
			}
		}
		if err != nil {
			return results, fmt.Errorf("migration %s failed: %w", f.name, err)
		}
		if err := st.Apply(f.index); err != nil {
			return results, fmt.Errorf("record apply %d: %w", f.index, err)
		}
		// Small delay to allow backend consistency before next migration
		time.Sleep(1 * time.Second)
	}
	return results, nil
}

// MigrateDown rolls back down to targetVersion (not including target): it will
// run downs for all applied versions > targetVersion in reverse order.
// Each successful down removes that version from the store.
func MigrateDown(ctx context.Context, dir string, baseEnv env.Env, targetVersion int) ([]*ExecWithVersion, error) {
	files, err := listMigrationFiles(dir)
	if err != nil {
		return nil, err
	}
	dbPath := filepath.Join(dir, store.DbFileName)
	st, err := store.Open(dbPath)
	if err != nil {
		return nil, err
	}
	defer func() { _ = st.Close() }()

	cur, err := st.CurrentVersion()
	if err != nil {
		return nil, err
	}
	if targetVersion < 0 {
		targetVersion = 0
	}
	if targetVersion > cur {
		return nil, fmt.Errorf("target version %d is above current %d", targetVersion, cur)
	}

	// map versions to files
	fileByVer := map[int]vfile{}
	for _, f := range files {
		fileByVer[f.index] = f
	}

	// collect applied versions to rollback: (target, cur]
	applied, err := st.ListApplied()
	if err != nil {
		return nil, err
	}
	toRollback := make([]int, 0)
	for _, v := range applied {
		if v > targetVersion {
			toRollback = append(toRollback, v)
		}
	}
	sort.Sort(sort.Reverse(sort.IntSlice(toRollback)))

	results := make([]*ExecWithVersion, 0, len(toRollback))
	for _, v := range toRollback {
		f, ok := fileByVer[v]
		if !ok {
			return results, fmt.Errorf("no migration file for version %d", v)
		}
		t, err := loadTaskFromFile(f.path)
		if err != nil {
			return results, fmt.Errorf("failed to load %s: %w", f.name, err)
		}
		if baseEnv.Global != nil {
			t.Down.Env.Global = baseEnv.Global
		} else {
			t.Down.Env.Global = map[string]string{}
		}
		if t.Down.Env.Local == nil {
			t.Down.Env.Local = map[string]string{}
		}
		// Merge stored env for this version into Down's Local env (prefer stored_env table; fallback to legacy env_json)
		if loaded, _ := st.LoadStoredEnv(v); len(loaded) > 0 {
			for k, val := range loaded {
				if _, exists := t.Down.Env.Local[k]; !exists {
					t.Down.Env.Local[k] = val
				}
			}
		} else if loadedLegacy, _ := st.LoadEnv(v, "up"); len(loadedLegacy) > 0 {
			for k, val := range loadedLegacy {
				if _, exists := t.Down.Env.Local[k]; !exists {
					t.Down.Env.Local[k] = val
				}
			}
		}
		res, err := t.DownExecute(ctx, "", "")
		results = append(results, &ExecWithVersion{Version: v, Result: res})
		if res != nil {
			save := false
			if vflag := ctx.Value(SaveResponseBodyKey); vflag != nil {
				if b, ok := vflag.(bool); ok {
					save = b
				}
			}
			var bodyPtr *string
			if save {
				b := res.ResponseBody
				bodyPtr = &b
			}
			_ = st.RecordRun(v, "down", res.StatusCode, bodyPtr, nil)
		}
		if err != nil {
			return results, fmt.Errorf("down %s failed: %w", f.name, err)
		}
		if err := st.Remove(v); err != nil {
			return results, fmt.Errorf("record remove %d: %w", v, err)
		}
		// Cleanup stored env rows for this version
		_ = st.DeleteStoredEnv(v)
	}
	return results, nil
}

// ExecWithVersion pairs ExecResult with version number.
type ExecWithVersion struct {
	Version int
	Result  *task.ExecResult
}
