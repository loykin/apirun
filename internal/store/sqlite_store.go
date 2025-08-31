package store

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

const DriverSqlite = "sqlite"

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
		s.DSN = "file:" + path + "?_busy_timeout=5000&_fk=1"
	}
	return nil
}

func (s *SqliteStore) Ensure(th TableNames) error {
	stmts := []string{
		fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s (version INTEGER PRIMARY KEY)", th.SchemaMigrations),
		fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s (id INTEGER PRIMARY KEY AUTOINCREMENT, version INTEGER NOT NULL, direction TEXT NOT NULL, status_code INTEGER NOT NULL, body TEXT NULL, env_json TEXT NULL, ran_at TEXT NOT NULL)", th.MigrationRuns),
		fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s (version INTEGER NOT NULL, name TEXT NOT NULL, value TEXT NOT NULL, PRIMARY KEY(version, name))", th.StoredEnv),
	}
	for _, q := range stmts {
		if _, err := s.db.Exec(q); err != nil {
			return err
		}
	}
	return nil
}

func (s *SqliteStore) Apply(th TableNames, v int) error {
	// #nosec G201 -- table name is validated via Store.safeTableNames (regex), values use placeholders
	q := fmt.Sprintf("INSERT OR IGNORE INTO %s(version) VALUES(?)", th.SchemaMigrations)
	_, err := s.db.Exec(q, v)
	return err
}

func (s *SqliteStore) IsApplied(th TableNames, v int) (bool, error) {
	// #nosec G201 -- table name is a validated identifier from safeTableNames; values are parameterized
	q := fmt.Sprintf("SELECT 1 FROM %s WHERE version = ?", th.SchemaMigrations)
	row := s.db.QueryRow(q, v)
	var one int
	err := row.Scan(&one)
	if err == sql.ErrNoRows {
		return false, nil
	}
	return err == nil, err
}

func (s *SqliteStore) CurrentVersion(th TableNames) (int, error) {
	// #nosec G201 -- only injecting validated table name; no user input; values are parameterized elsewhere
	q := fmt.Sprintf("SELECT COALESCE(MAX(version), 0) FROM %s", th.SchemaMigrations)
	row := s.db.QueryRow(q)
	var v int
	if err := row.Scan(&v); err != nil {
		return 0, err
	}
	return v, nil
}

func (s *SqliteStore) ListApplied(th TableNames) ([]int, error) {
	// #nosec G201 -- table identifier sanitized via safeTableNames; the query has no dynamic user data
	q := fmt.Sprintf("SELECT version FROM %s ORDER BY version ASC", th.SchemaMigrations)
	rows, err := s.db.Query(q)
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

func (s *SqliteStore) Remove(th TableNames, v int) error {
	// #nosec G201 -- validated table name; deletion predicate uses parameter placeholder
	q := fmt.Sprintf("DELETE FROM %s WHERE version = ?", th.SchemaMigrations)
	_, err := s.db.Exec(q, v)
	return err
}

func (s *SqliteStore) SetVersion(th TableNames, target int) error {
	cur, err := s.CurrentVersion(th)
	if err != nil {
		return err
	}
	if target == cur {
		return nil
	}
	if target > cur {
		return errors.New("cannot set version up; apply migrations instead")
	}
	// #nosec G201 -- table identifier validated via safeTableNames; comparison value is a bind parameter
	q := fmt.Sprintf("DELETE FROM %s WHERE version > ?", th.SchemaMigrations)
	_, err = s.db.Exec(q, target)
	return err
}

func (s *SqliteStore) RecordRun(th TableNames, version int, direction string, status int, body *string, env map[string]string) error {
	var envJSON *string
	if len(env) > 0 {
		b, _ := json.Marshal(env)
		s := string(b)
		envJSON = &s
	}
	ranAt := time.Now().UTC().Format(time.RFC3339Nano)
	// #nosec G201 -- table name is validated; values are fully parameterized with placeholders
	q := fmt.Sprintf("INSERT INTO %s(version, direction, status_code, body, env_json, ran_at) VALUES(?,?,?,?,?,?)", th.MigrationRuns)
	_, err := s.db.Exec(q, version, direction, status, body, envJSON, ranAt)
	return err
}

func (s *SqliteStore) LoadEnv(th TableNames, version int, direction string) (map[string]string, error) {
	// #nosec G201 -- only validated table name is interpolated; WHERE values are bound parameters
	q := fmt.Sprintf("SELECT env_json FROM %s WHERE version = ? AND direction = ? ORDER BY id DESC LIMIT 1", th.MigrationRuns)
	row := s.db.QueryRow(q, version, direction)
	var envJSON sql.NullString
	if err := row.Scan(&envJSON); err != nil {
		if err == sql.ErrNoRows {
			return map[string]string{}, nil
		}
		return map[string]string{}, err
	}
	if !envJSON.Valid || len(envJSON.String) == 0 {
		return map[string]string{}, nil
	}
	var out map[string]string
	if err := json.Unmarshal([]byte(envJSON.String), &out); err != nil {
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
			return err
		}
	}
	return nil
}

func (s *SqliteStore) LoadStoredEnv(th TableNames, version int) (map[string]string, error) {
	// #nosec G201 -- table name is sanitized; SELECT uses bound parameter for version
	q := fmt.Sprintf("SELECT name, value FROM %s WHERE version = ?", th.StoredEnv)
	rows, err := s.db.Query(q, version)
	if err != nil {
		return map[string]string{}, err
	}
	defer func() { _ = rows.Close() }()
	out := map[string]string{}
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			return map[string]string{}, err
		}
		out[k] = v
	}
	return out, rows.Err()
}

func (s *SqliteStore) DeleteStoredEnv(th TableNames, version int) error {
	// #nosec G201 -- validated table identifier; predicate value is parameterized
	q := fmt.Sprintf("DELETE FROM %s WHERE version = ?", th.StoredEnv)
	_, err := s.db.Exec(q, version)
	return err
}

func (s *SqliteStore) Connect() (*sql.DB, error) {
	if s.DSN == "" {
		// fallback to in-memory if not set explicitly (for safety)
		s.DSN = "file::memory:?cache=shared&_busy_timeout=5000&_fk=1"
	}
	db, err := sql.Open("sqlite", s.DSN)
	if err != nil {
		return nil, err
	}
	s.db = db
	return db, nil
}
