package store

import (
	"database/sql"
	"fmt"

	_ "github.com/jackc/pgx/v5/stdlib"
)

// pgSchema implements schema creation for PostgreSQL.
type pgSchema struct{}

func (pgSchema) Ensure(s *Store) error {
	// PostgreSQL schema
	if s.tn.schemaMigrations == "" {
		s.tn = defaultTableNames()
	}
	// schema_migrations
	if _, err := s.DB.Exec(fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
		version INTEGER PRIMARY KEY,
		applied_at TIMESTAMPTZ NOT NULL
	)`, s.tn.schemaMigrations)); err != nil {
		return err
	}
	// migration_runs
	if _, err := s.DB.Exec(fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
		id BIGSERIAL PRIMARY KEY,
		version INTEGER NOT NULL,
		direction TEXT NOT NULL,
		status_code INTEGER NOT NULL,
		body TEXT,
		env_json TEXT,
		ran_at TIMESTAMPTZ NOT NULL
	)`, s.tn.migrationRuns)); err != nil {
		return err
	}
	// stored_env
	if _, err := s.DB.Exec(fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
		id BIGSERIAL PRIMARY KEY,
		version INTEGER NOT NULL,
		name TEXT NOT NULL,
		value TEXT NOT NULL
	)`, s.tn.storedEnv)); err != nil {
		return err
	}
	return nil
}

// OpenPostgres opens a PostgreSQL-backed store using the given DSN.
// Example DSN: postgres://user:pass@host:5432/dbname
func OpenPostgres(dsn string) (*Store, error) {
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, err
	}
	st := &Store{DB: db, isPostgres: true, schema: pgSchema{}}
	st.tn = defaultTableNames()
	if err := st.EnsureSchema(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return st, nil
}

// OpenPostgresWithNames opens a PostgreSQL-backed store with custom table/index names set before schema creation.
func OpenPostgresWithNames(dsn string, schemaMigrations, migrationRuns, storedEnv, idxStoredEnvVersion string) (*Store, error) {
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, err
	}
	st := &Store{DB: db, isPostgres: true, schema: pgSchema{}}
	st.tn = defaultTableNames()
	st.SetTableNames(schemaMigrations, migrationRuns, storedEnv, idxStoredEnvVersion)
	if err := st.EnsureSchema(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return st, nil
}
