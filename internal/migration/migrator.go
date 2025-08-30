package migration

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/loykin/apimigrate/internal/auth"
	"github.com/loykin/apimigrate/internal/env"
	"github.com/loykin/apimigrate/internal/store"
	"github.com/loykin/apimigrate/internal/task"
	"gopkg.in/yaml.v3"
)

type Migrator struct {
	Dir        string
	Store      store.Store
	Env        env.Env
	tokenStore auth.TokenStore
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
		vr, toStore, err := runUpForFile(ctx, &m.Store, f, m.Env, sessionStored)
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
		vr, err := runDownForVersion(ctx, &m.Store, v, f, m.Env)
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
