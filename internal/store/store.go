package store

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
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

type schemaCreator interface {
	Ensure(*Store) error
}

type Store struct {
	DB         *sql.DB
	isPostgres bool
	schema     schemaCreator
	tn         tableNames
}

type tableNames struct {
	schemaMigrations    string
	migrationRuns       string
	storedEnv           string
	idxStoredEnvVersion string
}

var identRe = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`)

// safeTableNames returns validated table/index names; if a custom name is invalid,
// it falls back to the default for that identifier to avoid SQL injection via identifiers.
func (s *Store) safeTableNames() tableNames {
	d := defaultTableNames()
	t := s.tn
	if !identRe.MatchString(t.schemaMigrations) {
		t.schemaMigrations = d.schemaMigrations
	}
	if !identRe.MatchString(t.migrationRuns) {
		t.migrationRuns = d.migrationRuns
	}
	if !identRe.MatchString(t.storedEnv) {
		t.storedEnv = d.storedEnv
	}
	if !identRe.MatchString(t.idxStoredEnvVersion) {
		t.idxStoredEnvVersion = d.idxStoredEnvVersion
	}
	return t
}

func defaultTableNames() tableNames {
	return tableNames{
		schemaMigrations:    "schema_migrations",
		migrationRuns:       "migration_runs",
		storedEnv:           "stored_env",
		idxStoredEnvVersion: "idx_stored_env_version",
	}
}

// SetTableNames overrides the default table/index names. Empty values are ignored.
func (s *Store) SetTableNames(schemaMigrations, migrationRuns, storedEnv, idxStoredEnvVersion string) {
	if s == nil {
		return
	}
	if s.tn.schemaMigrations == "" {
		s.tn = defaultTableNames()
	}
	if schemaMigrations != "" {
		s.tn.schemaMigrations = schemaMigrations
	}
	if migrationRuns != "" {
		s.tn.migrationRuns = migrationRuns
	}
	if storedEnv != "" {
		s.tn.storedEnv = storedEnv
	}
	if idxStoredEnvVersion != "" {
		s.tn.idxStoredEnvVersion = idxStoredEnvVersion
	}
}

// conv converts '?' placeholders into PostgreSQL style $1, $2, ... when needed.
func (s *Store) conv(q string) string {
	if s == nil || !s.isPostgres {
		return q
	}
	res := make([]rune, 0, len(q)+8)
	idx := 1
	for _, r := range q {
		if r == '?' {
			ps := fmt.Sprintf("$%d", idx)
			for _, pr := range ps {
				res = append(res, pr)
			}
			idx++
			continue
		}
		res = append(res, r)
	}
	return string(res)
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
		tmp, _ := json.Marshal(env)
		envJSON = string(tmp)
	} else {
		envJSON = nil
	}
	tn := s.safeTableNames()
	q := s.conv(fmt.Sprintf("INSERT INTO %s(version, direction, status_code, body, env_json, ran_at) VALUES(?, ?, ?, ?, ?, ?)", tn.migrationRuns))
	_, err := s.DB.Exec(q, version, direction, statusCode, b, envJSON, time.Now().UTC().Format(time.RFC3339))
	return err
}

func Open(dbPath string) (*Store, error) {
	dsn := fmt.Sprintf("file:%s?_busy_timeout=5000&_fk=1", dbPath)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	st := &Store{DB: db, isPostgres: false, schema: sqliteSchema{}}
	st.tn = defaultTableNames()
	if err := st.EnsureSchema(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return st, nil
}

// OpenWithNames opens a SQLite store with custom table/index names applied before schema creation.
func OpenWithNames(dbPath string, schemaMigrations, migrationRuns, storedEnv, idxStoredEnvVersion string) (*Store, error) {
	dsn := fmt.Sprintf("file:%s?_busy_timeout=5000&_fk=1", dbPath)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	st := &Store{DB: db, isPostgres: false, schema: sqliteSchema{}}
	st.tn = defaultTableNames()
	st.SetTableNames(schemaMigrations, migrationRuns, storedEnv, idxStoredEnvVersion)
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
	if s == nil || s.DB == nil {
		return errors.New("nil store")
	}
	if s.schema == nil {
		return fmt.Errorf("no schema creator configured")
	}
	return s.schema.Ensure(s)
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
		if _, err = tx.Exec(s.conv(fmt.Sprintf("DELETE FROM %s WHERE version = ? AND name = ?", s.tn.storedEnv)), version, name); err != nil {
			return err
		}
		if _, err = tx.Exec(s.conv(fmt.Sprintf("INSERT INTO %s(version, name, value) VALUES(?, ?, ?)", s.tn.storedEnv)), version, name, val); err != nil {
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
	tn := s.safeTableNames()
	rows, err := s.DB.Query(s.conv(fmt.Sprintf("SELECT name, value FROM %s WHERE version = ?", tn.storedEnv)), version)
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
	tn := s.safeTableNames()
	_, err := s.DB.Exec(s.conv(fmt.Sprintf("DELETE FROM %s WHERE version = ?", tn.storedEnv)), version)
	return err
}

// Apply records that a migration version has been applied.
func (s *Store) Apply(version int) error {
	if s.isPostgres {
		tn := s.safeTableNames()
		q := fmt.Sprintf("INSERT INTO %s(version, applied_at) VALUES($1, $2) ON CONFLICT (version) DO NOTHING", tn.schemaMigrations)
		_, err := s.DB.Exec(q, version, time.Now().UTC().Format(time.RFC3339))
		return err
	}
	tn := s.safeTableNames()
	q := fmt.Sprintf("INSERT OR IGNORE INTO %s(version, applied_at) VALUES(?, ?)", tn.schemaMigrations)
	_, err := s.DB.Exec(q, version, time.Now().UTC().Format(time.RFC3339))
	return err
}

// Remove deletes a migration version record (used for down).
func (s *Store) Remove(version int) error {
	tn := s.safeTableNames()
	_, err := s.DB.Exec(s.conv(fmt.Sprintf("DELETE FROM %s WHERE version = ?", tn.schemaMigrations)), version)
	return err
}

// IsApplied returns true if the version exists in the table.
func (s *Store) IsApplied(version int) (bool, error) {
	tn := s.safeTableNames()
	row := s.DB.QueryRow(s.conv(fmt.Sprintf("SELECT 1 FROM %s WHERE version = ?", tn.schemaMigrations)), version)
	var one int
	err := row.Scan(&one)
	if err == sql.ErrNoRows {
		return false, nil
	}
	return err == nil, err
}

// CurrentVersion returns the highest applied version, or 0 if none.
func (s *Store) CurrentVersion() (int, error) {
	tn := s.safeTableNames()
	row := s.DB.QueryRow(fmt.Sprintf("SELECT COALESCE(MAX(version), 0) FROM %s", tn.schemaMigrations))
	var v int
	if err := row.Scan(&v); err != nil {
		return 0, err
	}
	return v, nil
}

// ListApplied returns applied versions sorted ascending.
func (s *Store) ListApplied() ([]int, error) {
	tn := s.safeTableNames()
	rows, err := s.DB.Query(fmt.Sprintf("SELECT version FROM %s ORDER BY version ASC", tn.schemaMigrations))
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
	tn := s.safeTableNames()
	_, err = s.DB.Exec(s.conv(fmt.Sprintf("DELETE FROM %s WHERE version > ?", tn.schemaMigrations)), target)
	return err
}

// LoadEnv returns the stored env map for a given version and direction from the latest run.
func (s *Store) LoadEnv(version int, direction string) (map[string]string, error) {
	if s == nil || s.DB == nil {
		return map[string]string{}, errors.New("nil store")
	}
	tn := s.safeTableNames()
	row := s.DB.QueryRow(s.conv(fmt.Sprintf("SELECT env_json FROM %s WHERE version = ? AND direction = ? ORDER BY id DESC LIMIT 1", tn.migrationRuns)), version, direction)
	var envJSON sql.NullString
	if err := row.Scan(&envJSON); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
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
