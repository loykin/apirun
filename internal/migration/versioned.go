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

// internal helpers to reduce cyclomatic complexity of MigrateUp/Down
func openStoreFromCtx(ctx context.Context, dir string) (*store.Store, error) {
	var (
		st  *store.Store
		err error
	)
	if v := ctx.Value(StoreOptionsKey); v != nil {
		if opts, ok := v.(*StoreOptions); ok && opts != nil {
			switch opts.Backend {
			case "postgres", "pg", "postgresql":
				if opts.PostgresDSN == "" {
					return nil, fmt.Errorf("store backend=postgres requires dsn")
				}
				if opts.TableSchemaMigrations != "" || opts.TableMigrationRuns != "" || opts.TableStoredEnv != "" || opts.IndexStoredEnvByVersion != "" {
					st, err = store.OpenPostgresWithNames(opts.PostgresDSN, opts.TableSchemaMigrations, opts.TableMigrationRuns, opts.TableStoredEnv, opts.IndexStoredEnvByVersion)
				} else {
					st, err = store.OpenPostgres(opts.PostgresDSN)
				}
			default:
				path := opts.SQLitePath
				if path == "" {
					path = filepath.Join(dir, store.DbFileName)
				}
				if opts.TableSchemaMigrations != "" || opts.TableMigrationRuns != "" || opts.TableStoredEnv != "" || opts.IndexStoredEnvByVersion != "" {
					st, err = store.OpenWithNames(path, opts.TableSchemaMigrations, opts.TableMigrationRuns, opts.TableStoredEnv, opts.IndexStoredEnvByVersion)
				} else {
					st, err = store.Open(path)
				}
			}
		} else {
			path := filepath.Join(dir, store.DbFileName)
			st, err = store.Open(path)
		}
	} else {
		path := filepath.Join(dir, store.DbFileName)
		st, err = store.Open(path)
	}
	return st, err
}

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
		return eww(ewv, toStore), toStore, err
	}
	return eww(ewv, nil), nil, err
}

// helper to ensure returned ExecWithVersion is passed through unchanged
func eww(v *ExecWithVersion, _ map[string]string) *ExecWithVersion { return v }

func MigrateUp(ctx context.Context, dir string, baseEnv env.Env, targetVersion int) ([]*ExecWithVersion, error) {
	files, err := listMigrationFiles(dir)
	if err != nil {
		return nil, err
	}
	// open store according to options (default sqlite under dir)
	st, err := openStoreFromCtx(ctx, dir)
	if err != nil {
		return nil, err
	}
	defer func() { _ = st.Close() }()

	cur, err := st.CurrentVersion()
	if err != nil {
		return nil, err
	}
	// plan versions to run
	plan := planUp(files, cur, targetVersion)

	results := make([]*ExecWithVersion, 0, len(plan))
	// sessionStored accumulates stored env created during this run to be available to later versions
	sessionStored := map[string]string{}
	for _, f := range plan {
		vr, toStore, err := runUpForFile(ctx, st, f, baseEnv, sessionStored)
		results = append(results, vr)
		for k, v := range toStore {
			sessionStored[k] = v
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
		return eww(ewv, nil), fmt.Errorf("down %s failed: %w", f.name, err)
	}
	if err := st.Remove(ver); err != nil {
		return eww(ewv, nil), fmt.Errorf("record remove %d: %w", ver, err)
	}
	_ = st.DeleteStoredEnv(ver)
	return eww(ewv, nil), nil
}

func MigrateDown(ctx context.Context, dir string, baseEnv env.Env, targetVersion int) ([]*ExecWithVersion, error) {
	files, err := listMigrationFiles(dir)
	if err != nil {
		return nil, err
	}
	// open store according to options (default sqlite under dir)
	st, err := openStoreFromCtx(ctx, dir)
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
	fileByVer := mapFilesByVersion(files)

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
		vr, err := runDownForVersion(ctx, st, v, f, baseEnv)
		results = append(results, vr)
		if err != nil {
			return results, err
		}
	}
	return results, nil
}

// ExecWithVersion pairs ExecResult with version number.
type ExecWithVersion struct {
	Version int
	Result  *task.ExecResult
}
