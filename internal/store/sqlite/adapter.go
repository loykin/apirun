package sqlite

import (
	"database/sql"

	"github.com/loykin/apirun/internal/store/connector"
)

// Adapter wraps sqlite.Store to implement connector.Connector interface
type Adapter struct {
	store *Store
}

// NewAdapter creates a new SQLite connector adapter
func NewAdapter() *Adapter {
	return &Adapter{
		store: NewStore(),
	}
}

func (a *Adapter) Connect() (*sql.DB, error) {
	return a.store.Connect()
}

func (a *Adapter) Validate() error {
	return a.store.Validate()
}

func (a *Adapter) Load(config map[string]interface{}) error {
	return a.store.Load(config)
}

func (a *Adapter) Ensure(th connector.TableNames) error {
	sqliteTh := TableNames{
		SchemaMigrations: th.SchemaMigrations,
		MigrationRuns:    th.MigrationRuns,
		StoredEnv:        th.StoredEnv,
	}
	return a.store.Ensure(sqliteTh)
}

func (a *Adapter) Apply(th connector.TableNames, v int) error {
	sqliteTh := TableNames{
		SchemaMigrations: th.SchemaMigrations,
		MigrationRuns:    th.MigrationRuns,
		StoredEnv:        th.StoredEnv,
	}
	return a.store.Apply(sqliteTh, v)
}

func (a *Adapter) IsApplied(th connector.TableNames, v int) (bool, error) {
	sqliteTh := TableNames{
		SchemaMigrations: th.SchemaMigrations,
		MigrationRuns:    th.MigrationRuns,
		StoredEnv:        th.StoredEnv,
	}
	return a.store.IsApplied(sqliteTh, v)
}

func (a *Adapter) CurrentVersion(th connector.TableNames) (int, error) {
	sqliteTh := TableNames{
		SchemaMigrations: th.SchemaMigrations,
		MigrationRuns:    th.MigrationRuns,
		StoredEnv:        th.StoredEnv,
	}
	return a.store.CurrentVersion(sqliteTh)
}

func (a *Adapter) ListApplied(th connector.TableNames) ([]int, error) {
	sqliteTh := TableNames{
		SchemaMigrations: th.SchemaMigrations,
		MigrationRuns:    th.MigrationRuns,
		StoredEnv:        th.StoredEnv,
	}
	return a.store.ListApplied(sqliteTh)
}

func (a *Adapter) Remove(th connector.TableNames, v int) error {
	sqliteTh := TableNames{
		SchemaMigrations: th.SchemaMigrations,
		MigrationRuns:    th.MigrationRuns,
		StoredEnv:        th.StoredEnv,
	}
	return a.store.Remove(sqliteTh, v)
}

func (a *Adapter) SetVersion(th connector.TableNames, target int) error {
	sqliteTh := TableNames{
		SchemaMigrations: th.SchemaMigrations,
		MigrationRuns:    th.MigrationRuns,
		StoredEnv:        th.StoredEnv,
	}
	return a.store.SetVersion(sqliteTh, target)
}

func (a *Adapter) RecordRun(th connector.TableNames, version int, direction string, status int, body *string, env map[string]string, failed bool) error {
	sqliteTh := TableNames{
		SchemaMigrations: th.SchemaMigrations,
		MigrationRuns:    th.MigrationRuns,
		StoredEnv:        th.StoredEnv,
	}
	return a.store.RecordRun(sqliteTh, version, direction, status, body, env, failed)
}

func (a *Adapter) LoadEnv(th connector.TableNames, version int, direction string) (map[string]string, error) {
	sqliteTh := TableNames{
		SchemaMigrations: th.SchemaMigrations,
		MigrationRuns:    th.MigrationRuns,
		StoredEnv:        th.StoredEnv,
	}
	return a.store.LoadEnv(sqliteTh, version, direction)
}

func (a *Adapter) InsertStoredEnv(th connector.TableNames, version int, kv map[string]string) error {
	sqliteTh := TableNames{
		SchemaMigrations: th.SchemaMigrations,
		MigrationRuns:    th.MigrationRuns,
		StoredEnv:        th.StoredEnv,
	}
	return a.store.InsertStoredEnv(sqliteTh, version, kv)
}

func (a *Adapter) LoadStoredEnv(th connector.TableNames, version int) (map[string]string, error) {
	sqliteTh := TableNames{
		SchemaMigrations: th.SchemaMigrations,
		MigrationRuns:    th.MigrationRuns,
		StoredEnv:        th.StoredEnv,
	}
	return a.store.LoadStoredEnv(sqliteTh, version)
}

func (a *Adapter) DeleteStoredEnv(th connector.TableNames, version int) error {
	sqliteTh := TableNames{
		SchemaMigrations: th.SchemaMigrations,
		MigrationRuns:    th.MigrationRuns,
		StoredEnv:        th.StoredEnv,
	}
	return a.store.DeleteStoredEnv(sqliteTh, version)
}

func (a *Adapter) ListRuns(th connector.TableNames) ([]connector.Run, error) {
	sqliteTh := TableNames{
		SchemaMigrations: th.SchemaMigrations,
		MigrationRuns:    th.MigrationRuns,
		StoredEnv:        th.StoredEnv,
	}
	sqliteRuns, err := a.store.ListRuns(sqliteTh)
	if err != nil {
		return nil, err
	}

	// Convert sqlite.Run to connector.Run
	runs := make([]connector.Run, len(sqliteRuns))
	for i, r := range sqliteRuns {
		runs[i] = connector.Run{
			ID:         r.ID,
			Version:    r.Version,
			Direction:  r.Direction,
			StatusCode: r.StatusCode,
			Body:       r.Body,
			Env:        r.Env,
			Failed:     r.Failed,
			RanAt:      r.RanAt,
		}
	}
	return runs, nil
}

func (a *Adapter) Close() error {
	return a.store.Close()
}
