package store

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

const DriverSqlite = "sqlite"

// SQLite configuration constants
const (
	sqliteBusyTimeoutMS    = 5000 // 5 seconds in milliseconds
	sqliteMaxOpenConns     = 1    // Maximum open connections for SQLite
	sqliteForeignKeysParam = "_fk=1"
	sqliteCacheSharedParam = "cache=shared"
)

type SqliteConfig struct {
	Path string
}

func (c *SqliteConfig) ToMap() map[string]interface{} {
	return map[string]interface{}{
		"path": c.Path,
	}
}

func NewSqliteConnector() Connector {
	connector := SqliteStore{}
	return &connector
}

type SqliteStore struct {
	DSN string
	db  *sql.DB
}

func (s *SqliteStore) Close() error {
	return s.db.Close()
}

func (s *SqliteStore) Validate() error { return nil }

func (s *SqliteStore) Load(config map[string]interface{}) error {
	if dsn, ok := config["dsn"].(string); ok && dsn != "" {
		s.DSN = dsn
		return nil
	}
	if path, ok := config["path"].(string); ok && path != "" {
		s.DSN = fmt.Sprintf("file:%s?_busy_timeout=%d&%s", path, sqliteBusyTimeoutMS, sqliteForeignKeysParam)
	}
	return nil
}

func (s *SqliteStore) Ensure(th TableNames) error {
	stmts := []string{
		fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s (version INTEGER PRIMARY KEY)", th.SchemaMigrations),
		fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s (id INTEGER PRIMARY KEY AUTOINCREMENT, version INTEGER NOT NULL, direction TEXT NOT NULL, status_code INTEGER NOT NULL, body TEXT NULL, env_json TEXT NULL, failed INTEGER NOT NULL DEFAULT 0, ran_at TEXT NOT NULL)", th.MigrationRuns),
		fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s (version INTEGER NOT NULL, name TEXT NOT NULL, value TEXT NOT NULL, PRIMARY KEY(version, name))", th.StoredEnv),
	}
	for i, q := range stmts {
		if _, err := s.db.Exec(q); err != nil {
			return fmt.Errorf("failed to create table %d in schema setup: %w", i+1, err)
		}
	}
	return nil
}

func (s *SqliteStore) Apply(th TableNames, v int) error {
	// #nosec G201 -- table name is validated via Store.safeTableNames (regex), values use placeholders
	q := fmt.Sprintf("INSERT OR IGNORE INTO %s(version) VALUES(?)", th.SchemaMigrations)
	_, err := s.db.Exec(q, v)
	if err != nil {
		return fmt.Errorf("failed to apply migration version %d: %w", v, err)
	}
	return nil
}

func (s *SqliteStore) IsApplied(th TableNames, v int) (bool, error) {
	// #nosec G201 -- table name is a validated identifier from safeTableNames; values are parameterized
	q := fmt.Sprintf("SELECT 1 FROM %s WHERE version = ?", th.SchemaMigrations)
	row := s.db.QueryRow(q, v)
	var one int
	err := row.Scan(&one)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("failed to check if migration version %d is applied: %w", v, err)
	}
	return true, nil
}

func (s *SqliteStore) CurrentVersion(th TableNames) (int, error) {
	// #nosec G201 -- only injecting validated table name; no user input; values are parameterized elsewhere
	q := fmt.Sprintf("SELECT COALESCE(MAX(version), 0) FROM %s", th.SchemaMigrations)
	row := s.db.QueryRow(q)
	var v int
	if err := row.Scan(&v); err != nil {
		return 0, fmt.Errorf("failed to get current migration version: %w", err)
	}
	return v, nil
}

func (s *SqliteStore) ListApplied(th TableNames) ([]int, error) {
	// #nosec G201 -- table identifier sanitized via safeTableNames; the query has no dynamic user data
	q := fmt.Sprintf("SELECT version FROM %s ORDER BY version ASC", th.SchemaMigrations)
	rows, err := s.db.Query(q)
	if err != nil {
		return nil, fmt.Errorf("failed to list applied migrations: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []int
	for rows.Next() {
		var v int
		if err := rows.Scan(&v); err != nil {
			return nil, fmt.Errorf("failed to scan migration version from database: %w", err)
		}
		out = append(out, v)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error occurred while iterating over applied migrations: %w", err)
	}
	return out, nil
}

func (s *SqliteStore) Remove(th TableNames, v int) error {
	// #nosec G201 -- validated table name; deletion predicate uses parameter placeholder
	q := fmt.Sprintf("DELETE FROM %s WHERE version = ?", th.SchemaMigrations)
	_, err := s.db.Exec(q, v)
	if err != nil {
		return fmt.Errorf("failed to remove migration version %d: %w", v, err)
	}
	return nil
}

func (s *SqliteStore) SetVersion(th TableNames, target int) error {
	cur, err := s.CurrentVersion(th)
	if err != nil {
		return fmt.Errorf("failed to get current version for SetVersion operation: %w", err)
	}
	if target == cur {
		return nil
	}
	if target > cur {
		return fmt.Errorf("cannot set version up from %d to %d; apply migrations instead", cur, target)
	}
	// #nosec G201 -- table identifier validated via safeTableNames; comparison value is a bind parameter
	q := fmt.Sprintf("DELETE FROM %s WHERE version > ?", th.SchemaMigrations)
	_, err = s.db.Exec(q, target)
	if err != nil {
		return fmt.Errorf("failed to set version to %d (was %d): %w", target, cur, err)
	}
	return nil
}

func (s *SqliteStore) RecordRun(th TableNames, version int, direction string, status int, body *string, env map[string]string, failed bool) error {
	var envJSON *string
	if len(env) > 0 {
		b, err := json.Marshal(env)
		if err != nil {
			return fmt.Errorf("failed to marshal environment for migration run record (version %d, direction %s): %w", version, direction, err)
		}
		s := string(b)
		envJSON = &s
	}
	ranAt := time.Now().UTC().Format(time.RFC3339Nano)
	// #nosec G201 -- table name is validated; values are fully parameterized with placeholders
	q := fmt.Sprintf("INSERT INTO %s(version, direction, status_code, body, env_json, failed, ran_at) VALUES(?,?,?,?,?,?,?)", th.MigrationRuns)
	failedInt := 0
	if failed {
		failedInt = 1
	}
	_, err := s.db.Exec(q, version, direction, status, body, envJSON, failedInt, ranAt)
	if err != nil {
		return fmt.Errorf("failed to record migration run (version %d, direction %s, status %d): %w", version, direction, status, err)
	}
	return nil
}

func (s *SqliteStore) LoadEnv(th TableNames, version int, direction string) (map[string]string, error) {
	// #nosec G201 -- only validated table name is interpolated; WHERE values are bound parameters
	q := fmt.Sprintf("SELECT env_json FROM %s WHERE version = ? AND direction = ? ORDER BY id DESC LIMIT 1", th.MigrationRuns)
	row := s.db.QueryRow(q, version, direction)
	var envJSON sql.NullString
	if err := row.Scan(&envJSON); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return map[string]string{}, nil
		}
		return map[string]string{}, fmt.Errorf("failed to load environment for migration version %d direction %s: %w", version, direction, err)
	}
	if !envJSON.Valid || len(envJSON.String) == 0 {
		return map[string]string{}, nil
	}
	var out map[string]string
	if err := json.Unmarshal([]byte(envJSON.String), &out); err != nil {
		// Log warning but don't fail - return empty map for compatibility
		return map[string]string{}, nil
	}
	return out, nil
}

func (s *SqliteStore) InsertStoredEnv(th TableNames, version int, kv map[string]string) error {
	if len(kv) == 0 {
		return nil
	}
	// #nosec G201 -- validated table identifier; UPSERT values provided only via placeholders
	q := fmt.Sprintf("INSERT INTO %s(version,name,value) VALUES(?,?,?) ON CONFLICT(version,name) DO UPDATE SET value=excluded.value", th.StoredEnv)
	for k, v := range kv {
		if _, err := s.db.Exec(q, version, k, v); err != nil {
			return fmt.Errorf("failed to insert stored environment variable %q for version %d: %w", k, version, err)
		}
	}
	return nil
}

func (s *SqliteStore) LoadStoredEnv(th TableNames, version int) (map[string]string, error) {
	// #nosec G201 -- table name is sanitized; SELECT uses bound parameter for version
	q := fmt.Sprintf("SELECT name, value FROM %s WHERE version = ?", th.StoredEnv)
	rows, err := s.db.Query(q, version)
	if err != nil {
		return map[string]string{}, fmt.Errorf("failed to load stored environment for version %d: %w", version, err)
	}
	defer func() { _ = rows.Close() }()
	out := map[string]string{}
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			return map[string]string{}, fmt.Errorf("failed to scan stored environment variable for version %d: %w", version, err)
		}
		out[k] = v
	}
	if err := rows.Err(); err != nil {
		return map[string]string{}, fmt.Errorf("error occurred while reading stored environment for version %d: %w", version, err)
	}
	return out, nil
}

func (s *SqliteStore) DeleteStoredEnv(th TableNames, version int) error {
	// #nosec G201 -- validated table identifier; predicate value is parameterized
	q := fmt.Sprintf("DELETE FROM %s WHERE version = ?", th.StoredEnv)
	_, err := s.db.Exec(q, version)
	if err != nil {
		return fmt.Errorf("failed to delete stored environment for version %d: %w", version, err)
	}
	return nil
}

func (s *SqliteStore) ListRuns(th TableNames) ([]Run, error) {
	// #nosec G201 -- only validated table name is interpolated; values scanned are plain
	q := fmt.Sprintf("SELECT id, version, direction, status_code, body, env_json, failed, ran_at FROM %s ORDER BY id ASC", th.MigrationRuns)
	rows, err := s.db.Query(q)
	if err != nil {
		return nil, fmt.Errorf("failed to query migration runs: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []Run
	for rows.Next() {
		var (
			id        int
			ver       int
			dir       string
			status    int
			body      sql.NullString
			envJSON   sql.NullString
			failedInt int
			ranAt     string
		)
		if err := rows.Scan(&id, &ver, &dir, &status, &body, &envJSON, &failedInt, &ranAt); err != nil {
			return nil, fmt.Errorf("failed to scan migration run record: %w", err)
		}
		var bptr *string
		if body.Valid {
			bv := body.String
			bptr = &bv
		}
		m := map[string]string{}
		if envJSON.Valid && envJSON.String != "" {
			_ = json.Unmarshal([]byte(envJSON.String), &m)
		}
		out = append(out, Run{ID: id, Version: ver, Direction: dir, StatusCode: status, Body: bptr, Env: m, Failed: failedInt != 0, RanAt: ranAt})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error occurred while reading migration runs: %w", err)
	}
	return out, nil
}

func (s *SqliteStore) Connect() (*sql.DB, error) {
	if s.DSN == "" {
		// fallback to in-memory if not set explicitly (for safety)
		s.DSN = fmt.Sprintf("file::memory:?%s&_busy_timeout=%d&%s", sqliteCacheSharedParam, sqliteBusyTimeoutMS, sqliteForeignKeysParam)
	}
	db, err := sql.Open("sqlite", s.DSN)
	if err != nil {
		return nil, fmt.Errorf("failed to open SQLite database with DSN %q: %w", s.DSN, err)
	}

	db.SetMaxOpenConns(sqliteMaxOpenConns)

	s.db = db
	return db, nil
}
