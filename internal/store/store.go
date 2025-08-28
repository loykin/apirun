package store

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

// DbFileName is the default filename for the migration history database.
const DbFileName = "apimigrate.db"

// Store persists migration versions in a SQLite database.
// Table schema_migrations(version INTEGER PRIMARY KEY, applied_at TEXT)
// Similar to goose semantics: version is the numeric prefix of the migration filename.
//
// DB path is a SQLite file path. Use Open(dbPath) to create/connect.

type Store struct {
	DB *sql.DB
}

// RecordRun inserts a row into migration_runs logging the status, optional response body, and optional env JSON.
func (s *Store) RecordRun(version int, direction string, statusCode int, body *string, env map[string]string) error {
	if s == nil || s.DB == nil {
		return errors.New("nil store")
	}
	var b interface{}
	if body != nil {
		b = *body
	} else {
		b = nil
	}
	var envJSON interface{}
	if len(env) > 0 {
		// Lazy marshal without adding a dep: use fmt and naive building? Better to use standard json
		// but json package is in stdlib; use it.
		// We will marshal to string and store.
		tmp, _ := json.Marshal(env)
		envJSON = string(tmp)
	} else {
		envJSON = nil
	}
	_, err := s.DB.Exec(`INSERT INTO migration_runs(version, direction, status_code, body, env_json, ran_at) VALUES(?, ?, ?, ?, ?, ?)`,
		version, direction, statusCode, b, envJSON, time.Now().UTC().Format(time.RFC3339))
	return err
}

func Open(dbPath string) (*Store, error) {
	dsn := fmt.Sprintf("file:%s?_busy_timeout=5000&_fk=1", dbPath)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	st := &Store{DB: db}
	if err := st.EnsureSchema(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return st, nil
}

func (s *Store) Close() error {
	if s == nil || s.DB == nil {
		return nil
	}
	return s.DB.Close()
}

func (s *Store) EnsureSchema() error {
	_, err := s.DB.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (
		version INTEGER PRIMARY KEY,
		applied_at TEXT NOT NULL
	)`)
	if err != nil {
		return err
	}
	_, err = s.DB.Exec(`CREATE TABLE IF NOT EXISTS migration_runs (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		version INTEGER NOT NULL,
		direction TEXT NOT NULL,
		status_code INTEGER NOT NULL,
		body TEXT,
		env_json TEXT,
		ran_at TEXT NOT NULL
	)`)
	if err != nil {
		return err
	}
	// Best-effort migration for existing tables missing env_json column
	_, _ = s.DB.Exec(`ALTER TABLE migration_runs ADD COLUMN env_json TEXT`)
	// New table to persist selected env entries per version
	_, err = s.DB.Exec(`CREATE TABLE IF NOT EXISTS stored_env (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		version INTEGER NOT NULL,
		name TEXT NOT NULL,
		value TEXT NOT NULL
	)`)
	if err != nil {
		return err
	}
	// Simple index to speed-up lookup by version
	_, _ = s.DB.Exec(`CREATE INDEX IF NOT EXISTS idx_stored_env_version ON stored_env(version)`)
	return nil
}

// InsertStoredEnv persists each name/value entry for a given version into stored_env.
func (s *Store) InsertStoredEnv(version int, kv map[string]string) error {
	if s == nil || s.DB == nil {
		return errors.New("nil store")
	}
	if len(kv) == 0 {
		return nil
	}
	// Best effort: remove any previous rows for this version/name to keep latest values
	// Use a transaction for atomicity
	tx, err := s.DB.Begin()
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()
	for name, val := range kv {
		// delete existing rows for same version+name
		if _, err = tx.Exec(`DELETE FROM stored_env WHERE version = ? AND name = ?`, version, name); err != nil {
			return err
		}
		if _, err = tx.Exec(`INSERT INTO stored_env(version, name, value) VALUES(?, ?, ?)`, version, name, val); err != nil {
			return err
		}
	}
	err = tx.Commit()
	return err
}

// LoadStoredEnv returns all name/value entries recorded for a given version.
func (s *Store) LoadStoredEnv(version int) (map[string]string, error) {
	if s == nil || s.DB == nil {
		return map[string]string{}, errors.New("nil store")
	}
	rows, err := s.DB.Query(`SELECT name, value FROM stored_env WHERE version = ?`, version)
	if err != nil {
		return map[string]string{}, err
	}
	defer func() { _ = rows.Close() }()
	m := map[string]string{}
	for rows.Next() {
		var name, val string
		if err := rows.Scan(&name, &val); err != nil {
			return map[string]string{}, err
		}
		m[name] = val
	}
	return m, rows.Err()
}

// DeleteStoredEnv removes all entries for the given version from stored_env.
func (s *Store) DeleteStoredEnv(version int) error {
	if s == nil || s.DB == nil {
		return errors.New("nil store")
	}
	_, err := s.DB.Exec(`DELETE FROM stored_env WHERE version = ?`, version)
	return err
}

// Apply records that a migration version has been applied.
func (s *Store) Apply(version int) error {
	_, err := s.DB.Exec(`INSERT OR IGNORE INTO schema_migrations(version, applied_at) VALUES(?, ?)`, version, time.Now().UTC().Format(time.RFC3339))
	return err
}

// Remove deletes a migration version record (used for down).
func (s *Store) Remove(version int) error {
	_, err := s.DB.Exec(`DELETE FROM schema_migrations WHERE version = ?`, version)
	return err
}

// IsApplied returns true if the version exists in the table.
func (s *Store) IsApplied(version int) (bool, error) {
	row := s.DB.QueryRow(`SELECT 1 FROM schema_migrations WHERE version = ?`, version)
	var one int
	err := row.Scan(&one)
	if err == sql.ErrNoRows {
		return false, nil
	}
	return err == nil, err
}

// CurrentVersion returns the highest applied version, or 0 if none.
func (s *Store) CurrentVersion() (int, error) {
	row := s.DB.QueryRow(`SELECT COALESCE(MAX(version), 0) FROM schema_migrations`)
	var v int
	if err := row.Scan(&v); err != nil {
		return 0, err
	}
	return v, nil
}

// ListApplied returns applied versions sorted ascending.
func (s *Store) ListApplied() ([]int, error) {
	rows, err := s.DB.Query(`SELECT version FROM schema_migrations ORDER BY version ASC`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []int
	for rows.Next() {
		var v int
		if err := rows.Scan(&v); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

// SetVersion sets the store to an exact version by applying/removing without executing migrations.
// This is dangerous and intended only for edge cases; prefer MigrateUp/MigrateDown flows.
func (s *Store) SetVersion(target int) error {
	cur, err := s.CurrentVersion()
	if err != nil {
		return err
	}
	if target == cur {
		return nil
	}
	if target > cur {
		return errors.New("SetVersion cannot move up; use MigrateUp")
	}
	// moving down: remove all versions > target
	_, err = s.DB.Exec(`DELETE FROM schema_migrations WHERE version > ?`, target)
	return err
}

// LoadEnv returns the stored env map for a given version and direction from the latest run.
func (s *Store) LoadEnv(version int, direction string) (map[string]string, error) {
	if s == nil || s.DB == nil {
		return map[string]string{}, errors.New("nil store")
	}
	row := s.DB.QueryRow(`SELECT env_json FROM migration_runs WHERE version = ? AND direction = ? ORDER BY id DESC LIMIT 1`, version, direction)
	var envJSON sql.NullString
	if err := row.Scan(&envJSON); err != nil {
		if err == sql.ErrNoRows {
			return map[string]string{}, nil
		}
		return map[string]string{}, err
	}
	if !envJSON.Valid || envJSON.String == "" {
		return map[string]string{}, nil
	}
	var m map[string]string
	if err := json.Unmarshal([]byte(envJSON.String), &m); err != nil {
		// If malformed, return empty rather than failing the migration
		return map[string]string{}, nil
	}
	return m, nil
}
