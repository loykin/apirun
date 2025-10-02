package postgresql

import (
	"database/sql"

	"github.com/loykin/apirun/internal/store/connector"
)

// Adapter wraps postgresql.Store to implement connector.Connector interface
type Adapter struct {
	store *Store
}

// NewAdapter creates a new PostgreSQL connector adapter
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
	postgresTh := TableNames{
		SchemaMigrations: th.SchemaMigrations,
		MigrationRuns:    th.MigrationRuns,
		StoredEnv:        th.StoredEnv,
	}
	return a.store.Ensure(postgresTh)
}

func (a *Adapter) Apply(th connector.TableNames, v int) error {
	postgresTh := TableNames{
		SchemaMigrations: th.SchemaMigrations,
		MigrationRuns:    th.MigrationRuns,
		StoredEnv:        th.StoredEnv,
	}
	return a.store.Apply(postgresTh, v)
}

func (a *Adapter) IsApplied(th connector.TableNames, v int) (bool, error) {
	postgresTh := TableNames{
		SchemaMigrations: th.SchemaMigrations,
		MigrationRuns:    th.MigrationRuns,
		StoredEnv:        th.StoredEnv,
	}
	return a.store.IsApplied(postgresTh, v)
}

func (a *Adapter) CurrentVersion(th connector.TableNames) (int, error) {
	postgresTh := TableNames{
		SchemaMigrations: th.SchemaMigrations,
		MigrationRuns:    th.MigrationRuns,
		StoredEnv:        th.StoredEnv,
	}
	return a.store.CurrentVersion(postgresTh)
}

func (a *Adapter) ListApplied(th connector.TableNames) ([]int, error) {
	postgresTh := TableNames{
		SchemaMigrations: th.SchemaMigrations,
		MigrationRuns:    th.MigrationRuns,
		StoredEnv:        th.StoredEnv,
	}
	return a.store.ListApplied(postgresTh)
}

func (a *Adapter) Remove(th connector.TableNames, v int) error {
	postgresTh := TableNames{
		SchemaMigrations: th.SchemaMigrations,
		MigrationRuns:    th.MigrationRuns,
		StoredEnv:        th.StoredEnv,
	}
	return a.store.Remove(postgresTh, v)
}

func (a *Adapter) SetVersion(th connector.TableNames, target int) error {
	postgresTh := TableNames{
		SchemaMigrations: th.SchemaMigrations,
		MigrationRuns:    th.MigrationRuns,
		StoredEnv:        th.StoredEnv,
	}
	return a.store.SetVersion(postgresTh, target)
}

func (a *Adapter) RecordRun(th connector.TableNames, version int, direction string, status int, body *string, env map[string]string, failed bool) error {
	postgresTh := TableNames{
		SchemaMigrations: th.SchemaMigrations,
		MigrationRuns:    th.MigrationRuns,
		StoredEnv:        th.StoredEnv,
	}
	return a.store.RecordRun(postgresTh, version, direction, status, body, env, failed)
}

func (a *Adapter) LoadEnv(th connector.TableNames, version int, direction string) (map[string]string, error) {
	postgresTh := TableNames{
		SchemaMigrations: th.SchemaMigrations,
		MigrationRuns:    th.MigrationRuns,
		StoredEnv:        th.StoredEnv,
	}
	return a.store.LoadEnv(postgresTh, version, direction)
}

func (a *Adapter) InsertStoredEnv(th connector.TableNames, version int, kv map[string]string) error {
	postgresTh := TableNames{
		SchemaMigrations: th.SchemaMigrations,
		MigrationRuns:    th.MigrationRuns,
		StoredEnv:        th.StoredEnv,
	}
	return a.store.InsertStoredEnv(postgresTh, version, kv)
}

func (a *Adapter) LoadStoredEnv(th connector.TableNames, version int) (map[string]string, error) {
	postgresTh := TableNames{
		SchemaMigrations: th.SchemaMigrations,
		MigrationRuns:    th.MigrationRuns,
		StoredEnv:        th.StoredEnv,
	}
	return a.store.LoadStoredEnv(postgresTh, version)
}

func (a *Adapter) DeleteStoredEnv(th connector.TableNames, version int) error {
	postgresTh := TableNames{
		SchemaMigrations: th.SchemaMigrations,
		MigrationRuns:    th.MigrationRuns,
		StoredEnv:        th.StoredEnv,
	}
	return a.store.DeleteStoredEnv(postgresTh, version)
}

func (a *Adapter) ListRuns(th connector.TableNames) ([]connector.Run, error) {
	postgresTh := TableNames{
		SchemaMigrations: th.SchemaMigrations,
		MigrationRuns:    th.MigrationRuns,
		StoredEnv:        th.StoredEnv,
	}
	postgresRuns, err := a.store.ListRuns(postgresTh)
	if err != nil {
		return nil, err
	}

	// Convert postgresql.Run to connector.Run
	runs := make([]connector.Run, len(postgresRuns))
	for i, r := range postgresRuns {
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
