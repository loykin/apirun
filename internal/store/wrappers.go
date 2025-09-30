package store

import (
	"database/sql"

	"github.com/loykin/apirun/internal/store/postgresql"
	"github.com/loykin/apirun/internal/store/sqlite"
)

// sqliteConnectorWrapper wraps sqlite.Store to implement Connector interface
type sqliteConnectorWrapper struct {
	store *sqlite.Store
}

func (w *sqliteConnectorWrapper) Connect() (*sql.DB, error) {
	return w.store.Connect()
}

func (w *sqliteConnectorWrapper) Validate() error {
	return w.store.Validate()
}

func (w *sqliteConnectorWrapper) Load(config map[string]interface{}) error {
	return w.store.Load(config)
}

func (w *sqliteConnectorWrapper) Ensure(th TableNames) error {
	sqliteTh := sqlite.TableNames{
		SchemaMigrations: th.SchemaMigrations,
		MigrationRuns:    th.MigrationRuns,
		StoredEnv:        th.StoredEnv,
	}
	return w.store.Ensure(sqliteTh)
}

func (w *sqliteConnectorWrapper) Apply(th TableNames, v int) error {
	sqliteTh := sqlite.TableNames{
		SchemaMigrations: th.SchemaMigrations,
		MigrationRuns:    th.MigrationRuns,
		StoredEnv:        th.StoredEnv,
	}
	return w.store.Apply(sqliteTh, v)
}

func (w *sqliteConnectorWrapper) IsApplied(th TableNames, v int) (bool, error) {
	sqliteTh := sqlite.TableNames{
		SchemaMigrations: th.SchemaMigrations,
		MigrationRuns:    th.MigrationRuns,
		StoredEnv:        th.StoredEnv,
	}
	return w.store.IsApplied(sqliteTh, v)
}

func (w *sqliteConnectorWrapper) CurrentVersion(th TableNames) (int, error) {
	sqliteTh := sqlite.TableNames{
		SchemaMigrations: th.SchemaMigrations,
		MigrationRuns:    th.MigrationRuns,
		StoredEnv:        th.StoredEnv,
	}
	return w.store.CurrentVersion(sqliteTh)
}

func (w *sqliteConnectorWrapper) ListApplied(th TableNames) ([]int, error) {
	sqliteTh := sqlite.TableNames{
		SchemaMigrations: th.SchemaMigrations,
		MigrationRuns:    th.MigrationRuns,
		StoredEnv:        th.StoredEnv,
	}
	return w.store.ListApplied(sqliteTh)
}

func (w *sqliteConnectorWrapper) Remove(th TableNames, v int) error {
	sqliteTh := sqlite.TableNames{
		SchemaMigrations: th.SchemaMigrations,
		MigrationRuns:    th.MigrationRuns,
		StoredEnv:        th.StoredEnv,
	}
	return w.store.Remove(sqliteTh, v)
}

func (w *sqliteConnectorWrapper) SetVersion(th TableNames, target int) error {
	sqliteTh := sqlite.TableNames{
		SchemaMigrations: th.SchemaMigrations,
		MigrationRuns:    th.MigrationRuns,
		StoredEnv:        th.StoredEnv,
	}
	return w.store.SetVersion(sqliteTh, target)
}

func (w *sqliteConnectorWrapper) RecordRun(th TableNames, version int, direction string, status int, body *string, env map[string]string, failed bool) error {
	sqliteTh := sqlite.TableNames{
		SchemaMigrations: th.SchemaMigrations,
		MigrationRuns:    th.MigrationRuns,
		StoredEnv:        th.StoredEnv,
	}
	return w.store.RecordRun(sqliteTh, version, direction, status, body, env, failed)
}

func (w *sqliteConnectorWrapper) LoadEnv(th TableNames, version int, direction string) (map[string]string, error) {
	sqliteTh := sqlite.TableNames{
		SchemaMigrations: th.SchemaMigrations,
		MigrationRuns:    th.MigrationRuns,
		StoredEnv:        th.StoredEnv,
	}
	return w.store.LoadEnv(sqliteTh, version, direction)
}

func (w *sqliteConnectorWrapper) InsertStoredEnv(th TableNames, version int, kv map[string]string) error {
	sqliteTh := sqlite.TableNames{
		SchemaMigrations: th.SchemaMigrations,
		MigrationRuns:    th.MigrationRuns,
		StoredEnv:        th.StoredEnv,
	}
	return w.store.InsertStoredEnv(sqliteTh, version, kv)
}

func (w *sqliteConnectorWrapper) LoadStoredEnv(th TableNames, version int) (map[string]string, error) {
	sqliteTh := sqlite.TableNames{
		SchemaMigrations: th.SchemaMigrations,
		MigrationRuns:    th.MigrationRuns,
		StoredEnv:        th.StoredEnv,
	}
	return w.store.LoadStoredEnv(sqliteTh, version)
}

func (w *sqliteConnectorWrapper) DeleteStoredEnv(th TableNames, version int) error {
	sqliteTh := sqlite.TableNames{
		SchemaMigrations: th.SchemaMigrations,
		MigrationRuns:    th.MigrationRuns,
		StoredEnv:        th.StoredEnv,
	}
	return w.store.DeleteStoredEnv(sqliteTh, version)
}

func (w *sqliteConnectorWrapper) ListRuns(th TableNames) ([]Run, error) {
	sqliteTh := sqlite.TableNames{
		SchemaMigrations: th.SchemaMigrations,
		MigrationRuns:    th.MigrationRuns,
		StoredEnv:        th.StoredEnv,
	}
	sqliteRuns, err := w.store.ListRuns(sqliteTh)
	if err != nil {
		return nil, err
	}

	// Convert sqlite.Run to store.Run
	runs := make([]Run, len(sqliteRuns))
	for i, r := range sqliteRuns {
		runs[i] = Run{
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

func (w *sqliteConnectorWrapper) Close() error {
	return w.store.Close()
}

// postgresConnectorWrapper wraps postgresql.Store to implement Connector interface
type postgresConnectorWrapper struct {
	store *postgresql.Store
}

func (w *postgresConnectorWrapper) Connect() (*sql.DB, error) {
	return w.store.Connect()
}

func (w *postgresConnectorWrapper) Validate() error {
	return w.store.Validate()
}

func (w *postgresConnectorWrapper) Load(config map[string]interface{}) error {
	return w.store.Load(config)
}

func (w *postgresConnectorWrapper) Ensure(th TableNames) error {
	postgresTh := postgresql.TableNames{
		SchemaMigrations: th.SchemaMigrations,
		MigrationRuns:    th.MigrationRuns,
		StoredEnv:        th.StoredEnv,
	}
	return w.store.Ensure(postgresTh)
}

func (w *postgresConnectorWrapper) Apply(th TableNames, v int) error {
	postgresTh := postgresql.TableNames{
		SchemaMigrations: th.SchemaMigrations,
		MigrationRuns:    th.MigrationRuns,
		StoredEnv:        th.StoredEnv,
	}
	return w.store.Apply(postgresTh, v)
}

func (w *postgresConnectorWrapper) IsApplied(th TableNames, v int) (bool, error) {
	postgresTh := postgresql.TableNames{
		SchemaMigrations: th.SchemaMigrations,
		MigrationRuns:    th.MigrationRuns,
		StoredEnv:        th.StoredEnv,
	}
	return w.store.IsApplied(postgresTh, v)
}

func (w *postgresConnectorWrapper) CurrentVersion(th TableNames) (int, error) {
	postgresTh := postgresql.TableNames{
		SchemaMigrations: th.SchemaMigrations,
		MigrationRuns:    th.MigrationRuns,
		StoredEnv:        th.StoredEnv,
	}
	return w.store.CurrentVersion(postgresTh)
}

func (w *postgresConnectorWrapper) ListApplied(th TableNames) ([]int, error) {
	postgresTh := postgresql.TableNames{
		SchemaMigrations: th.SchemaMigrations,
		MigrationRuns:    th.MigrationRuns,
		StoredEnv:        th.StoredEnv,
	}
	return w.store.ListApplied(postgresTh)
}

func (w *postgresConnectorWrapper) Remove(th TableNames, v int) error {
	postgresTh := postgresql.TableNames{
		SchemaMigrations: th.SchemaMigrations,
		MigrationRuns:    th.MigrationRuns,
		StoredEnv:        th.StoredEnv,
	}
	return w.store.Remove(postgresTh, v)
}

func (w *postgresConnectorWrapper) SetVersion(th TableNames, target int) error {
	postgresTh := postgresql.TableNames{
		SchemaMigrations: th.SchemaMigrations,
		MigrationRuns:    th.MigrationRuns,
		StoredEnv:        th.StoredEnv,
	}
	return w.store.SetVersion(postgresTh, target)
}

func (w *postgresConnectorWrapper) RecordRun(th TableNames, version int, direction string, status int, body *string, env map[string]string, failed bool) error {
	postgresTh := postgresql.TableNames{
		SchemaMigrations: th.SchemaMigrations,
		MigrationRuns:    th.MigrationRuns,
		StoredEnv:        th.StoredEnv,
	}
	return w.store.RecordRun(postgresTh, version, direction, status, body, env, failed)
}

func (w *postgresConnectorWrapper) LoadEnv(th TableNames, version int, direction string) (map[string]string, error) {
	postgresTh := postgresql.TableNames{
		SchemaMigrations: th.SchemaMigrations,
		MigrationRuns:    th.MigrationRuns,
		StoredEnv:        th.StoredEnv,
	}
	return w.store.LoadEnv(postgresTh, version, direction)
}

func (w *postgresConnectorWrapper) InsertStoredEnv(th TableNames, version int, kv map[string]string) error {
	postgresTh := postgresql.TableNames{
		SchemaMigrations: th.SchemaMigrations,
		MigrationRuns:    th.MigrationRuns,
		StoredEnv:        th.StoredEnv,
	}
	return w.store.InsertStoredEnv(postgresTh, version, kv)
}

func (w *postgresConnectorWrapper) LoadStoredEnv(th TableNames, version int) (map[string]string, error) {
	postgresTh := postgresql.TableNames{
		SchemaMigrations: th.SchemaMigrations,
		MigrationRuns:    th.MigrationRuns,
		StoredEnv:        th.StoredEnv,
	}
	return w.store.LoadStoredEnv(postgresTh, version)
}

func (w *postgresConnectorWrapper) DeleteStoredEnv(th TableNames, version int) error {
	postgresTh := postgresql.TableNames{
		SchemaMigrations: th.SchemaMigrations,
		MigrationRuns:    th.MigrationRuns,
		StoredEnv:        th.StoredEnv,
	}
	return w.store.DeleteStoredEnv(postgresTh, version)
}

func (w *postgresConnectorWrapper) ListRuns(th TableNames) ([]Run, error) {
	postgresTh := postgresql.TableNames{
		SchemaMigrations: th.SchemaMigrations,
		MigrationRuns:    th.MigrationRuns,
		StoredEnv:        th.StoredEnv,
	}
	postgresRuns, err := w.store.ListRuns(postgresTh)
	if err != nil {
		return nil, err
	}

	// Convert postgresql.Run to store.Run
	runs := make([]Run, len(postgresRuns))
	for i, r := range postgresRuns {
		runs[i] = Run{
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

func (w *postgresConnectorWrapper) Close() error {
	return w.store.Close()
}
