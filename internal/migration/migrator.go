package migration

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/loykin/apimigrate/internal/env"
	"github.com/loykin/apimigrate/internal/store"
	"github.com/loykin/apimigrate/internal/task"
	"gopkg.in/yaml.v3"
)

type Migrator struct {
	Dir              string
	Store            store.Store
	Env              env.Env
	SaveResponseBody bool
}

// MigrateUp applies migrations greater than the current store version up to targetVersion.
// If targetVersion <= 0, it applies all pending migrations.
// It records each applied version in the store after successful execution.
func (m *Migrator) runUpForFile(ctx context.Context, f vfile, sessionStored map[string]string) (*ExecWithVersion, map[string]string, error) {
	t, err := loadTaskFromFile(f.path)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to load %s: %w", f.name, err)
	}
	if m.Env.Global != nil {
		t.Up.Env.Global = m.Env.Global
	} else {
		t.Up.Env.Global = map[string]string{}
	}
	// propagate auth map for template rendering ({{.auth.name}})
	if m.Env.Auth != nil {
		t.Up.Env.Auth = m.Env.Auth
	} else {
		t.Up.Env.Auth = map[string]string{}
	}
	if t.Up.Env.Local == nil {
		t.Up.Env.Local = map[string]string{}
	}
	// Merge stored env from previously applied versions
	if applied, err := m.Store.ListApplied(); err == nil {
		for _, av := range applied {
			if m2, _ := m.Store.LoadStoredEnv(av); len(m2) > 0 {
				for k, val := range m2 {
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
		save := m.SaveResponseBody
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
		_ = m.Store.RecordRun(f.index, "up", res.StatusCode, bodyPtr, toStore)
		_ = m.Store.InsertStoredEnv(f.index, toStore)
		return ewv, toStore, err
	}
	return ewv, nil, err
}

// MigrateDown rolls back down to targetVersion (not including target): it will
// run downs for all applied versions > targetVersion in reverse order.
// Each successful down removes that version from the store.
func (m *Migrator) runDownForVersion(ctx context.Context, ver int, f vfile) (*ExecWithVersion, error) {
	t, err := loadTaskFromFile(f.path)
	if err != nil {
		return nil, fmt.Errorf("failed to load %s: %w", f.name, err)
	}
	if m.Env.Global != nil {
		t.Down.Env.Global = m.Env.Global
	} else {
		t.Down.Env.Global = map[string]string{}
	}
	// propagate auth map for template rendering ({{.auth.name}})
	if m.Env.Auth != nil {
		t.Down.Env.Auth = m.Env.Auth
	} else {
		t.Down.Env.Auth = map[string]string{}
	}
	if t.Down.Env.Local == nil {
		t.Down.Env.Local = map[string]string{}
	}
	// Merge stored env for this version (prefer stored_env; fallback to legacy)
	if loaded, _ := m.Store.LoadStoredEnv(ver); len(loaded) > 0 {
		for k, val := range loaded {
			if _, exists := t.Down.Env.Local[k]; !exists {
				t.Down.Env.Local[k] = val
			}
		}
	} else if loadedLegacy, _ := m.Store.LoadEnv(ver, "up"); len(loadedLegacy) > 0 {
		for k, val := range loadedLegacy {
			if _, exists := t.Down.Env.Local[k]; !exists {
				t.Down.Env.Local[k] = val
			}
		}
	}
	res, err := t.DownExecute(ctx, "", "")
	ewv := &ExecWithVersion{Version: ver, Result: res}
	if res != nil {
		save := m.SaveResponseBody
		var bodyPtr *string
		if save {
			b := res.ResponseBody
			bodyPtr = &b
		}
		_ = m.Store.RecordRun(ver, "down", res.StatusCode, bodyPtr, nil)
	}
	if err != nil {
		return ewv, fmt.Errorf("down %s failed: %w", f.name, err)
	}
	if err := m.Store.Remove(ver); err != nil {
		return ewv, fmt.Errorf("record remove %d: %w", ver, err)
	}
	_ = m.Store.DeleteStoredEnv(ver)
	return ewv, nil
}

func (m *Migrator) MigrateUp(ctx context.Context, targetVersion int) ([]*ExecWithVersion, error) {
	files, err := listMigrationFiles(m.Dir)
	if err != nil {
		return nil, err
	}

	cur, err := m.Store.CurrentVersion()
	if err != nil {
		return nil, err
	}
	// plan versions to run
	plan := planUp(files, cur, targetVersion)

	results := make([]*ExecWithVersion, 0, len(plan))
	// sessionStored accumulates stored env created during this run to be available to later versions
	sessionStored := map[string]string{}
	for _, f := range plan {
		vr, toStore, err := m.runUpForFile(ctx, f, sessionStored)
		results = append(results, vr)
		for k, v := range toStore {
			sessionStored[k] = v
		}
		if err != nil {
			return results, fmt.Errorf("migration %s failed: %w", f.name, err)
		}
		if err := m.Store.Apply(f.index); err != nil {
			return results, fmt.Errorf("record apply %d: %w", f.index, err)
		}
		// Small delay to allow backend consistency before next migration
		time.Sleep(1 * time.Second)
	}
	return results, nil
}

func (m *Migrator) MigrateDown(ctx context.Context, targetVersion int) ([]*ExecWithVersion, error) {
	files, err := listMigrationFiles(m.Dir)
	if err != nil {
		return nil, err
	}

	cur, err := m.Store.CurrentVersion()
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
	applied, err := m.Store.ListApplied()
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
		vr, err := m.runDownForVersion(ctx, v, f)
		results = append(results, vr)
		if err != nil {
			return results, err
		}
	}
	return results, nil
}

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
