package store

import "fmt"

// sqliteSchema implements schema creation for SQLite.
type sqliteSchema struct{}

func (sqliteSchema) Ensure(s *Store) error {
	// SQLite schema
	if s.tn.schemaMigrations == "" {
		s.tn = defaultTableNames()
	}
	tn := s.safeTableNames()
	if _, err := s.DB.Exec(fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
		version INTEGER PRIMARY KEY,
		applied_at TEXT NOT NULL
	)`, tn.schemaMigrations)); err != nil {
		return err
	}
	if _, err := s.DB.Exec(fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		version INTEGER NOT NULL,
		direction TEXT NOT NULL,
		status_code INTEGER NOT NULL,
		body TEXT,
		env_json TEXT,
		ran_at TEXT NOT NULL
	)`, tn.migrationRuns)); err != nil {
		return err
	}
	// Ensure env_json column exists for legacy DBs (best-effort)
	_, _ = s.DB.Exec(fmt.Sprintf(`ALTER TABLE %s ADD COLUMN env_json TEXT`, tn.migrationRuns))
	if _, err := s.DB.Exec(fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		version INTEGER NOT NULL,
		name TEXT NOT NULL,
		value TEXT NOT NULL
	)`, tn.storedEnv)); err != nil {
		return err
	}
	return nil
}
