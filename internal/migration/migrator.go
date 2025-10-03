package migration

import (
	"context"
	"crypto/tls"
	"fmt"
	"sort"
	"time"

	"github.com/loykin/apirun/internal/auth"
	acommon "github.com/loykin/apirun/internal/auth/common"
	"github.com/loykin/apirun/internal/common"
	"github.com/loykin/apirun/internal/store"
	"github.com/loykin/apirun/internal/task"
	"github.com/loykin/apirun/pkg/env"
)

type Migrator struct {
	Dir              string
	Store            store.Store
	Env              *env.Env
	Auth             []auth.Auth
	SaveResponseBody bool
	// RenderBodyDefault controls default templating for RequestSpec bodies when not set per-request.
	// nil means default to true (render). When false, bodies with templates like {{...}} are sent as-is, unrendered.
	RenderBodyDefault *bool
	// DryRun disables store mutations and simulates applied versions based on DryRunFrom.
	DryRun bool
	// DryRunFrom represents the snapshot version already applied when DryRun is true.
	// 0 means from the beginning; N means treat versions <= N as applied.
	DryRunFrom int
	// TLSConfig applies to all HTTP requests executed by tasks during migrations.
	TLSConfig *tls.Config
	// DelayBetweenMigrations configures the delay between migration executions for backend consistency.
	// If not set, defaults to 1 second. Set to 0 to disable delays.
	DelayBetweenMigrations time.Duration
}

// getDelayBetweenMigrations returns the configured delay or default value
func (m *Migrator) getDelayBetweenMigrations() time.Duration {
	if m.DelayBetweenMigrations > 0 {
		return m.DelayBetweenMigrations
	}
	// Default to 1 second for backward compatibility
	return 1 * time.Second
}

// contextSleep sleeps for the given duration or until context is cancelled
func contextSleep(ctx context.Context, duration time.Duration) error {
	timer := time.NewTimer(duration)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

// initTaskAndEnv loads task from file and initializes env for up/down, merges stored/session env as needed.
func (m *Migrator) initTaskAndEnv(t *task.Task, f vfile, ver int, sessionStored map[string]string, mode string) error {
	if err := t.LoadFromFile(f.path); err != nil {
		return fmt.Errorf("failed to load %s: %w", f.name, err)
	}
	if mode == "up" {
		// prepare up env
		t.Up.Env = m.prepareTaskEnv(t.Up.Env)
		// Merge stored env from previously applied versions
		var applied []int
		if m.DryRun {
			for i := 1; i <= m.DryRunFrom; i++ {
				applied = append(applied, i)
			}
		} else if list, err := m.Store.ListApplied(); err == nil {
			applied = list
		}
		if len(applied) > 0 {
			for _, av := range applied {
				if m2, _ := m.Store.LoadStoredEnv(av); len(m2) > 0 {
					for k, val := range m2 {
						if _, exists := t.Up.Env.Local[k]; !exists {
							t.Up.Env.Local[k] = env.Str(val)
						}
					}
				}
			}
		}
		// Merge sessionStored
		if len(sessionStored) > 0 {
			for k, val := range sessionStored {
				if _, exists := t.Up.Env.Local[k]; !exists {
					t.Up.Env.Local[k] = env.Str(val)
				}
			}
		}
		// Apply global default for body rendering if request didn't set explicitly
		if t.Up.Request.RenderBody == nil && m.RenderBodyDefault != nil {
			val := *m.RenderBodyDefault
			t.Up.Request.RenderBody = &val
		}
		return nil
	}
	// down mode
	t.Down.Env = m.prepareTaskEnv(t.Down.Env)
	// Merge stored env for this version (prefer stored_env; fallback to legacy up env)
	if loaded, _ := m.Store.LoadStoredEnv(ver); len(loaded) > 0 {
		for k, val := range loaded {
			if _, exists := t.Down.Env.Local[k]; !exists {
				t.Down.Env.Local[k] = env.Str(val)
			}
		}
	} else if loadedLegacy, _ := m.Store.LoadEnv(ver, "up"); len(loadedLegacy) > 0 {
		for k, val := range loadedLegacy {
			if _, exists := t.Down.Env.Local[k]; !exists {
				t.Down.Env.Local[k] = env.Str(val)
			}
		}
	}
	// Apply global default for body rendering on optional Find.Request if not set
	if t.Down.Find != nil && t.Down.Find.Request.RenderBody == nil && m.RenderBodyDefault != nil {
		val := *m.RenderBodyDefault
		t.Down.Find.Request.RenderBody = &val
	}
	return nil
}

// prepareTaskEnv returns a per-task environment initialized from the Migrator base env.
// It guarantees non-nil Env and maps for Auth/Global/Local. Global/Auth are copied from m.Env.Clone().
func (m *Migrator) prepareTaskEnv(current *env.Env) *env.Env {
	// Start with a concrete env instance
	if current == nil {
		current = env.New()
	}
	// Clone base (nil-safe) and copy maps
	cl := (*env.Env)(nil)
	if m != nil {
		cl = m.Env.Clone()
	} else {
		cl = env.New()
	}
	if cl.Global != nil {
		current.Global = cl.Global
	} else if current.Global == nil {
		current.Global = env.Map{}
	}
	if cl.Auth != nil {
		current.Auth = cl.Auth
	} else if current.Auth == nil {
		current.Auth = env.Map{}
	}
	if cl.Local != nil {
		current.Local = cl.Local
	} else if current.Local == nil {
		current.Local = env.Map{}
	}
	return current
}

// ensureAuth wires lazy acquisition for configured auth entries instead of acquiring immediately.
// It prepares Env.AuthAcquire and pre-fills Env.Auth with empty values for referenced names so that
// templates like {{.auth.name}} trigger acquisition on demand. Existing non-empty Env.Auth values are kept.
func (m *Migrator) ensureAuth(ctx context.Context) error {
	if m == nil || m.Auth == nil || len(m.Auth) == 0 {
		return nil
	}
	// Ensure Env and Auth map exist and install lazy values for each configured auth
	if m.Env == nil {
		m.Env = env.New()
	}
	if m.Env.Auth == nil {
		m.Env.Auth = env.Map{}
	}
	for i := range m.Auth {
		a := m.Auth[i]
		name := a.Name
		if name == "" {
			continue
		}
		// Keep non-empty preset if already provided; otherwise set lazy
		if v, ok := m.Env.Auth[name]; ok {
			if v != nil && v.String() != "" {
				continue
			}
		}
		// Directly call provider from the lazy resolver without intermediate procs map
		authCfg := a
		m.Env.Auth[name] = m.Env.MakeLazy(func(e *env.Env) (string, error) {
			cctx := ctx
			if cctx == nil {
				cctx = context.Background()
			}
			return authCfg.Acquire(cctx, e)
		})
	}
	return nil
}

// MigrateUp applies migrations greater than the current store version up to targetVersion.
// If targetVersion <= 0, it applies all pending migrations.
// It records each applied version in the store after successful execution.
func (m *Migrator) runUpForFile(ctx context.Context, f vfile, sessionStored map[string]string) (*ExecWithVersion, map[string]string, error) {
	var t task.Task
	if err := m.initTaskAndEnv(&t, f, f.index, sessionStored, "up"); err != nil {
		return nil, nil, fmt.Errorf("failed to initialize task for migration version %d: %w", f.index, err)
	}
	res, err := t.Up.Execute(ctx, "", "")
	ewv := &ExecWithVersion{Version: f.index, Result: res}
	if res != nil {
		save := m.SaveResponseBody
		var bodyPtr *string
		if save {
			b := res.ResponseBody
			bodyPtr = &b
		}
		toStore := map[string]string{}
		if res.ExtractedEnv != nil {
			toStore = res.ExtractedEnv
		}
		if !m.DryRun {
			_ = m.Store.RecordRun(f.index, "up", res.StatusCode, bodyPtr, toStore, err != nil)
			_ = m.Store.InsertStoredEnv(f.index, toStore)
		}
		if err != nil {
			return ewv, toStore, fmt.Errorf("migration version %d execution failed: %w", f.index, err)
		}
		return ewv, toStore, nil
	}
	if err != nil {
		return ewv, nil, fmt.Errorf("migration version %d failed with no result: %w", f.index, err)
	}
	return ewv, nil, nil
}

// MigrateDown rolls back down to targetVersion (not including target): it will
// run downs for all applied versions > targetVersion in reverse order.
// Each successful down removes that version from the store.
func (m *Migrator) runDownForVersion(ctx context.Context, ver int, f vfile) (*ExecWithVersion, error) {
	var t task.Task
	if err := m.initTaskAndEnv(&t, f, ver, nil, "down"); err != nil {
		return nil, err
	}
	res, err := t.Down.Execute(ctx)
	ewv := &ExecWithVersion{Version: ver, Result: res}
	if res != nil {
		save := m.SaveResponseBody
		var bodyPtr *string
		if save {
			b := res.ResponseBody
			bodyPtr = &b
		}
		if !m.DryRun {
			_ = m.Store.RecordRun(ver, "down", res.StatusCode, bodyPtr, nil, err != nil)
		}
	}
	if err != nil {
		return ewv, fmt.Errorf("down %s failed: %w", f.name, err)
	}
	if !m.DryRun {
		if err := m.Store.Remove(ver); err != nil {
			return ewv, fmt.Errorf("record remove %d: %w", ver, err)
		}
		_ = m.Store.DeleteStoredEnv(ver)
	}
	return ewv, nil
}

func (m *Migrator) MigrateUp(ctx context.Context, targetVersion int) ([]*ExecWithVersion, error) {
	logger := common.GetLogger().WithComponent("migrator")
	startTime := time.Now()
	logger.Info("starting migration up",
		"target_version", targetVersion,
		"dir", m.Dir,
		"dry_run", m.DryRun)

	// Apply TLS settings for task HTTP requests and auth providers
	task.SetTLSConfig(m.TLSConfig)
	acommon.SetTLSConfig(m.TLSConfig)
	// Perform automatic auth once if configured
	if err := m.ensureAuth(ctx); err != nil {
		logger.Error("failed to ensure authentication", "error", err)
		return nil, fmt.Errorf("failed to ensure authentication: %w", err)
	}
	files, err := listMigrationFiles(m.Dir)
	if err != nil {
		logger.Error("failed to list migration files", "error", err, "dir", m.Dir)
		return nil, fmt.Errorf("failed to list migration files in directory %q: %w", m.Dir, err)
	}
	logger.Debug("found migration files", "count", len(files), "files", files)

	var cur int
	if m.DryRun {
		cur = m.DryRunFrom
		logger.Debug("dry run mode enabled", "dry_run_from", cur)
	} else {
		var err error
		cur, err = m.Store.CurrentVersion()
		if err != nil {
			logger.Error("failed to get current migration version from store", "error", err)
			return nil, fmt.Errorf("failed to get current migration version from store: %w", err)
		}
	}
	logger.Debug("current migration version", "version", cur)
	// plan versions to run
	plan := planUp(files, cur, targetVersion)

	results := make([]*ExecWithVersion, 0, len(plan))
	// sessionStored accumulates stored env created during this run to be available to later versions
	sessionStored := map[string]string{}
	for _, f := range plan {
		logger.Info("applying migration",
			"version", f.index,
			"file", f.name)
		vr, toStore, err := m.runUpForFile(ctx, f, sessionStored)
		results = append(results, vr)
		for k, v := range toStore {
			sessionStored[k] = v
		}
		if err != nil {
			return results, fmt.Errorf("migration %s failed: %w", f.name, err)
		}
		if !m.DryRun {
			if err := m.Store.Apply(f.index); err != nil {
				return results, fmt.Errorf("record apply %d: %w", f.index, err)
			}
			// Configurable delay to allow backend consistency before next migration
			delay := m.getDelayBetweenMigrations()
			if delay > 0 {
				if err := contextSleep(ctx, delay); err != nil {
					logger.Warn("migration delay interrupted by context cancellation", "error", err)
					return results, err
				}
			}
		}
	}

	duration := time.Since(startTime)
	logger.Info("migration up completed",
		"applied_count", len(results),
		"duration_ms", duration.Milliseconds(),
		"dry_run", m.DryRun)
	return results, nil
}

func (m *Migrator) MigrateDown(ctx context.Context, targetVersion int) ([]*ExecWithVersion, error) {
	logger := common.GetLogger().WithComponent("migrator")
	startTime := time.Now()
	logger.Info("starting migration down",
		"target_version", targetVersion,
		"dir", m.Dir,
		"dry_run", m.DryRun)

	// Apply TLS settings for task HTTP requests and auth providers
	task.SetTLSConfig(m.TLSConfig)
	acommon.SetTLSConfig(m.TLSConfig)
	// Perform automatic auth once if configured
	if err := m.ensureAuth(ctx); err != nil {
		return nil, fmt.Errorf("failed to ensure authentication for down migration: %w", err)
	}
	files, err := listMigrationFiles(m.Dir)
	if err != nil {
		return nil, fmt.Errorf("failed to list migration files in directory %q for down migration: %w", m.Dir, err)
	}

	var cur int
	if m.DryRun {
		cur = m.DryRunFrom
	} else {
		var err error
		cur, err = m.Store.CurrentVersion()
		if err != nil {
			return nil, fmt.Errorf("failed to get current migration version from store for down migration: %w", err)
		}
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
	var applied []int
	if m.DryRun {
		// simulate applied 1..cur
		for i := 1; i <= cur; i++ {
			applied = append(applied, i)
		}
	} else {
		var err error
		applied, err = m.Store.ListApplied()
		if err != nil {
			return nil, err
		}
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
		logger.Info("rolling back migration",
			"version", v,
			"file", f.name)
		vr, err := m.runDownForVersion(ctx, v, f)
		results = append(results, vr)
		if err != nil {
			return results, err
		}
	}

	duration := time.Since(startTime)
	logger.Info("migration down completed",
		"rolled_back_count", len(results),
		"duration_ms", duration.Milliseconds(),
		"dry_run", m.DryRun)
	return results, nil
}
