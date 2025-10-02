package sqlite

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/loykin/apirun/internal/common"
)

// Run represents a single execution record from the migration_runs table.
type Run struct {
	ID         int
	Version    int
	Direction  string
	StatusCode int
	Body       *string
	Env        map[string]string
	Failed     bool
	RanAt      string
}

// TableNames represents database table names
type TableNames struct {
	SchemaMigrations string
	MigrationRuns    string
	StoredEnv        string
}

type Store struct {
	db      *sql.DB
	dialect *Dialect
	DSN     string
}

// NewStore creates a new SQLite store
func NewStore() *Store {
	return &Store{
		dialect: NewDialect(),
	}
}

// Load loads configuration into the SQLite store
func (s *Store) Load(config map[string]interface{}) error {
	if dsn, ok := config["dsn"].(string); ok && dsn != "" {
		s.DSN = dsn
		return nil
	}
	if path, ok := config["path"].(string); ok && path != "" {
		s.DSN = fmt.Sprintf("file:%s?_busy_timeout=%d&%s", path, busyTimeoutMS, foreignKeysParam)
	}
	return nil
}

// Connect establishes a connection to SQLite using the adapter
func (s *Store) Connect() (*sql.DB, error) {
	if s.DSN == "" {
		// Default to in-memory database for testing
		s.DSN = ":memory:"
	}

	db, err := s.dialect.Connect(s.DSN)
	if err != nil {
		return nil, err
	}
	s.db = db

	logger := common.GetLogger().WithStore("sqlite")
	logger.Info("SQLite database connection established successfully")
	return db, nil
}

// Validate performs basic validation (default implementation)
func (s *Store) Validate() error {
	return nil
}

// Close closes the database connection
func (s *Store) Close() error {
	if s.db != nil {
		return s.db.Close()
	}
	return nil
}

// Ensure creates the necessary tables using SQLite-specific schema
func (s *Store) Ensure(th TableNames) error {
	logger := common.GetLogger().WithStore("sqlite")
	logger.Debug("ensuring SQLite database schema", "tables", []string{th.SchemaMigrations, th.MigrationRuns, th.StoredEnv})

	stmts := s.dialect.GetEnsureStatements(th.SchemaMigrations, th.MigrationRuns, th.StoredEnv)
	for i, q := range stmts {
		logger.Debug("executing schema creation statement", "table_index", i+1, "sql", q)
		if _, err := s.db.Exec(q); err != nil {
			logger.Error("failed to create table in schema setup", "error", err, "table_index", i+1, "sql", q)
			return fmt.Errorf("failed to create table %d in schema setup: %w", i+1, err)
		}
	}
	logger.Info("SQLite database schema ensured successfully")
	return nil
}

// Apply inserts a migration version into the schema_migrations table
func (s *Store) Apply(th TableNames, v int) error {
	logger := common.GetLogger().WithStore(s.dialect.GetDriverName()).WithVersion(v)
	logger.Debug("applying migration version")

	var q string
	upsertClause := s.dialect.GetUpsertClause()
	if upsertClause != "" {
		q = fmt.Sprintf("INSERT %s INTO %s(version) VALUES(%s)",
			upsertClause, th.SchemaMigrations, s.dialect.GetPlaceholder())
	} else {
		q = fmt.Sprintf("INSERT INTO %s(version) VALUES(%s) ON CONFLICT DO NOTHING",
			th.SchemaMigrations, s.dialect.GetPlaceholder())
	}

	_, err := s.db.Exec(q, v)
	if err != nil {
		logger.Error("failed to apply migration version", "error", err)
		return fmt.Errorf("failed to apply migration version %d: %w", v, err)
	}

	logger.Info("migration version applied successfully")
	return nil
}

// IsApplied checks if a migration version has been applied
func (s *Store) IsApplied(th TableNames, v int) (bool, error) {
	logger := common.GetLogger().WithStore(s.dialect.GetDriverName()).WithVersion(v)
	logger.Debug("checking if migration version is applied")

	q := fmt.Sprintf("SELECT 1 FROM %s WHERE version = %s", th.SchemaMigrations, s.dialect.GetPlaceholder())

	var result int
	err := s.db.QueryRow(q, v).Scan(&result)
	if errors.Is(err, sql.ErrNoRows) {
		logger.Debug("migration version not applied")
		return false, nil
	}
	if err != nil {
		logger.Error("failed to check if migration is applied", "error", err)
		return false, fmt.Errorf("failed to check if migration %d is applied: %w", v, err)
	}

	logger.Debug("migration version is applied")
	return true, nil
}

// CurrentVersion returns the highest applied migration version
func (s *Store) CurrentVersion(th TableNames) (int, error) {
	q := fmt.Sprintf("SELECT COALESCE(MAX(version), 0) FROM %s", th.SchemaMigrations)

	var version int
	err := s.db.QueryRow(q).Scan(&version)
	if err != nil {
		return 0, fmt.Errorf("failed to get current version: %w", err)
	}
	return version, nil
}

// ListApplied returns a list of all applied migration versions
func (s *Store) ListApplied(th TableNames) ([]int, error) {
	q := fmt.Sprintf("SELECT version FROM %s ORDER BY version", th.SchemaMigrations)

	rows, err := s.db.Query(q)
	if err != nil {
		return nil, fmt.Errorf("failed to list applied migrations: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var versions []int
	for rows.Next() {
		var version int
		if err := rows.Scan(&version); err != nil {
			return nil, fmt.Errorf("failed to scan migration version: %w", err)
		}
		versions = append(versions, version)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating migration versions: %w", err)
	}
	return versions, nil
}

// Remove removes a migration version from the schema_migrations table
func (s *Store) Remove(th TableNames, v int) error {
	q := fmt.Sprintf("DELETE FROM %s WHERE version = %s", th.SchemaMigrations, s.dialect.GetPlaceholder())

	_, err := s.db.Exec(q, v)
	if err != nil {
		return fmt.Errorf("failed to remove migration version %d: %w", v, err)
	}
	return nil
}

// SetVersion sets the schema to a specific version by removing all versions above the target
func (s *Store) SetVersion(th TableNames, target int) error {
	current, err := s.CurrentVersion(th)
	if err != nil {
		return err
	}

	if target > current {
		return fmt.Errorf("target version %d is greater than current version %d", target, current)
	}

	if target == current {
		return nil
	}

	q := fmt.Sprintf("DELETE FROM %s WHERE version > %s", th.SchemaMigrations, s.dialect.GetPlaceholder())

	_, err = s.db.Exec(q, target)
	if err != nil {
		return fmt.Errorf("failed to set version to %d: %w", target, err)
	}
	return nil
}

// LoadEnv loads environment variables from a migration run record
func (s *Store) LoadEnv(th TableNames, version int, direction string) (map[string]string, error) {
	q := fmt.Sprintf("SELECT env_json FROM %s WHERE version = %s AND direction = %s ORDER BY id DESC LIMIT 1",
		th.MigrationRuns, s.dialect.GetPlaceholder(), s.dialect.GetPlaceholder())

	var envJSON *string
	err := s.db.QueryRow(q, version, direction).Scan(&envJSON)
	if errors.Is(err, sql.ErrNoRows) {
		return map[string]string{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to load env for version %d direction %s: %w", version, direction, err)
	}

	if envJSON == nil || *envJSON == "" {
		return map[string]string{}, nil
	}

	var env map[string]string
	if err := json.Unmarshal([]byte(*envJSON), &env); err != nil {
		return map[string]string{}, nil
	}
	return env, nil
}

// LoadStoredEnv loads stored environment variables for a specific version
func (s *Store) LoadStoredEnv(th TableNames, version int) (map[string]string, error) {
	q := fmt.Sprintf("SELECT name, value FROM %s WHERE version = %s", th.StoredEnv, s.dialect.GetPlaceholder())

	rows, err := s.db.Query(q, version)
	if err != nil {
		return nil, fmt.Errorf("failed to load stored env for version %d: %w", version, err)
	}
	defer func() { _ = rows.Close() }()

	env := make(map[string]string)
	for rows.Next() {
		var name, value string
		if err := rows.Scan(&name, &value); err != nil {
			return nil, fmt.Errorf("failed to scan stored env: %w", err)
		}
		env[name] = value
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating stored env: %w", err)
	}
	return env, nil
}

// DeleteStoredEnv deletes stored environment variables for a specific version
func (s *Store) DeleteStoredEnv(th TableNames, version int) error {
	q := fmt.Sprintf("DELETE FROM %s WHERE version = %s", th.StoredEnv, s.dialect.GetPlaceholder())

	_, err := s.db.Exec(q, version)
	if err != nil {
		return fmt.Errorf("failed to delete stored env for version %d: %w", version, err)
	}
	return nil
}

// RecordRun records a migration run with SQLite-specific type handling
func (s *Store) RecordRun(th TableNames, version int, direction string, status int, body *string, env map[string]string, failed bool) error {
	logger := common.GetLogger().WithStore("sqlite").WithVersion(version)
	logger.Debug("recording migration run", "direction", direction, "status", status, "failed", failed)

	var envJSON *string
	if len(env) > 0 {
		b, err := json.Marshal(env)
		if err != nil {
			logger.Error("failed to marshal environment", "error", err)
			return fmt.Errorf("failed to marshal environment for migration run record (version %d, direction %s): %w", version, direction, err)
		}
		s := string(b)
		envJSON = &s
	}

	ranAt := s.dialect.ConvertTimeToStorage(time.Now().UTC())
	failedVal := s.dialect.ConvertBoolToStorage(failed)

	q := fmt.Sprintf("INSERT INTO %s(version, direction, status_code, body, env_json, failed, ran_at) VALUES(%s,%s,%s,%s,%s,%s,%s)",
		th.MigrationRuns,
		s.dialect.GetPlaceholder(), s.dialect.GetPlaceholder(), s.dialect.GetPlaceholder(),
		s.dialect.GetPlaceholder(), s.dialect.GetPlaceholder(), s.dialect.GetPlaceholder(),
		s.dialect.GetPlaceholder())

	_, err := s.db.Exec(q, version, direction, status, body, envJSON, failedVal, ranAt)
	if err != nil {
		logger.Error("failed to record migration run", "error", err)
		return fmt.Errorf("failed to record migration run (version %d, direction %s, status %d): %w", version, direction, status, err)
	}

	logger.Info("migration run recorded successfully", "direction", direction)
	return nil
}

// InsertStoredEnv inserts stored environment variables
func (s *Store) InsertStoredEnv(th TableNames, version int, kv map[string]string) error {
	// maxStoredEnvVars is set safely below math.MaxInt/3 to prevent overflow in capacity calculation (len(kv)*3)
	const maxStoredEnvVars = 10000
	logger := common.GetLogger().WithStore("sqlite").WithVersion(version)
	logger.Debug("inserting stored environment variables", "count", len(kv))

	if len(kv) == 0 {
		return nil
	}

	if len(kv) < 0 || len(kv) > maxStoredEnvVars {
		err := fmt.Errorf("cannot store more than %d environment variables (got: %d)", maxStoredEnvVars, len(kv))
		logger.Error("too many environment variables", "max", maxStoredEnvVars, "got", len(kv), "error", err)
		return err
	}

	valuesClauses := make([]string, 0, len(kv))
	capacity := len(kv) * 3
	args := make([]interface{}, 0, capacity)

	for name, value := range kv {
		valuesClauses = append(valuesClauses, "(?,?,?)")
		args = append(args, version, name, value)
	}

	q := fmt.Sprintf("INSERT OR REPLACE INTO %s(version, name, value) VALUES %s",
		th.StoredEnv, strings.Join(valuesClauses, ","))

	_, err := s.db.Exec(q, args...)
	if err != nil {
		logger.Error("failed to insert stored environment", "error", err)
		return fmt.Errorf("failed to insert stored environment for version %d: %w", version, err)
	}

	logger.Info("stored environment variables inserted successfully")
	return nil
}

// ListRuns returns migration run history with SQLite-specific type handling
func (s *Store) ListRuns(th TableNames) ([]Run, error) {
	logger := common.GetLogger().WithStore("sqlite")
	logger.Debug("listing migration runs")

	q := fmt.Sprintf("SELECT id, version, direction, status_code, body, env_json, failed, ran_at FROM %s ORDER BY id ASC", th.MigrationRuns)

	rows, err := s.db.Query(q)
	if err != nil {
		logger.Error("failed to query migration runs", "error", err)
		return nil, fmt.Errorf("failed to list migration runs: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var runs []Run
	for rows.Next() {
		var run Run
		var body sql.NullString
		var envJSON sql.NullString
		var ranAt string
		var failed int64

		err := rows.Scan(&run.ID, &run.Version, &run.Direction, &run.StatusCode, &body, &envJSON, &failed, &ranAt)
		if err != nil {
			logger.Error("failed to scan migration run", "error", err)
			return nil, fmt.Errorf("failed to scan migration run: %w", err)
		}

		if body.Valid {
			run.Body = &body.String
		}
		if envJSON.Valid && envJSON.String != "" {
			var envMap map[string]string
			if err := json.Unmarshal([]byte(envJSON.String), &envMap); err == nil {
				run.Env = envMap
			}
		}
		if run.Env == nil {
			run.Env = make(map[string]string)
		}

		run.Failed = s.dialect.ConvertBoolFromStorage(failed)
		run.RanAt = s.dialect.ConvertTimeFromStorage(ranAt)

		runs = append(runs, run)
	}

	if err := rows.Err(); err != nil {
		logger.Error("error iterating migration runs", "error", err)
		return nil, fmt.Errorf("error iterating migration runs: %w", err)
	}

	logger.Debug("migration runs listed successfully", "count", len(runs))
	return runs, nil
}
