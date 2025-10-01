package postgresql

import (
	"database/sql"
	"encoding/json"
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

// NewStore creates a new PostgreSQL store
func NewStore() *Store {
	return &Store{
		dialect: NewDialect(),
	}
}

// Load loads configuration into the PostgreSQL store
func (p *Store) Load(config map[string]interface{}) error {
	if dsn, ok := config["dsn"].(string); ok && dsn != "" {
		p.DSN = dsn
	}
	return nil
}

// Connect establishes a connection to PostgreSQL using the adapter
func (p *Store) Connect() (*sql.DB, error) {
	db, err := p.dialect.Connect(p.DSN)
	if err != nil {
		return nil, err
	}
	p.db = db

	logger := common.GetLogger().WithStore("postgresql")
	logger.Info("PostgreSQL database connection established successfully")
	return db, nil
}

// Validate performs basic validation (default implementation)
func (p *Store) Validate() error {
	return nil
}

// Close closes the database connection
func (p *Store) Close() error {
	if p.db != nil {
		return p.db.Close()
	}
	return nil
}

// Ensure creates the necessary tables using PostgreSQL-specific schema
func (p *Store) Ensure(th TableNames) error {
	logger := common.GetLogger().WithStore("postgresql")
	logger.Debug("ensuring PostgreSQL database schema", "tables", []string{th.SchemaMigrations, th.MigrationRuns, th.StoredEnv})

	stmts := p.dialect.GetEnsureStatements(th.SchemaMigrations, th.MigrationRuns, th.StoredEnv)
	for i, q := range stmts {
		logger.Debug("executing schema creation statement", "table_index", i+1, "sql", q)
		if _, err := p.db.Exec(q); err != nil {
			logger.Error("failed to create table in schema setup", "error", err, "table_index", i+1, "sql", q)
			return fmt.Errorf("failed to create table %d in PostgreSQL schema setup: %w", i+1, err)
		}
	}

	logger.Info("PostgreSQL database schema ensured successfully")
	return nil
}

// Apply inserts a migration version into the schema_migrations table
func (p *Store) Apply(th TableNames, v int) error {
	logger := common.GetLogger().WithStore(p.dialect.GetDriverName()).WithVersion(v)
	logger.Debug("applying migration version")

	var q string
	upsertClause := p.dialect.GetUpsertClause()
	if upsertClause != "" {
		q = fmt.Sprintf("INSERT %s INTO %s(version) VALUES(%s)",
			upsertClause, th.SchemaMigrations, p.dialect.GetPlaceholder(1))
	} else {
		q = fmt.Sprintf("INSERT INTO %s(version) VALUES(%s) ON CONFLICT DO NOTHING",
			th.SchemaMigrations, p.dialect.GetPlaceholder(1))
	}

	_, err := p.db.Exec(q, v)
	if err != nil {
		logger.Error("failed to apply migration version", "error", err)
		return fmt.Errorf("failed to apply migration version %d: %w", v, err)
	}

	logger.Info("migration version applied successfully")
	return nil
}

// IsApplied checks if a migration version has been applied
func (p *Store) IsApplied(th TableNames, v int) (bool, error) {
	logger := common.GetLogger().WithStore(p.dialect.GetDriverName()).WithVersion(v)
	logger.Debug("checking if migration version is applied")

	q := fmt.Sprintf("SELECT 1 FROM %s WHERE version = %s", th.SchemaMigrations, p.dialect.GetPlaceholder(1))

	var result int
	err := p.db.QueryRow(q, v).Scan(&result)
	if err == sql.ErrNoRows {
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
func (p *Store) CurrentVersion(th TableNames) (int, error) {
	q := fmt.Sprintf("SELECT COALESCE(MAX(version), 0) FROM %s", th.SchemaMigrations)

	var version int
	err := p.db.QueryRow(q).Scan(&version)
	if err != nil {
		return 0, fmt.Errorf("failed to get current version: %w", err)
	}
	return version, nil
}

// ListApplied returns a list of all applied migration versions
func (p *Store) ListApplied(th TableNames) ([]int, error) {
	q := fmt.Sprintf("SELECT version FROM %s ORDER BY version", th.SchemaMigrations)

	rows, err := p.db.Query(q)
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
func (p *Store) Remove(th TableNames, v int) error {
	q := fmt.Sprintf("DELETE FROM %s WHERE version = %s", th.SchemaMigrations, p.dialect.GetPlaceholder(1))

	_, err := p.db.Exec(q, v)
	if err != nil {
		return fmt.Errorf("failed to remove migration version %d: %w", v, err)
	}
	return nil
}

// SetVersion sets the schema to a specific version by removing all versions above the target
func (p *Store) SetVersion(th TableNames, target int) error {
	current, err := p.CurrentVersion(th)
	if err != nil {
		return err
	}

	if target > current {
		return fmt.Errorf("target version %d is greater than current version %d", target, current)
	}

	if target == current {
		return nil
	}

	q := fmt.Sprintf("DELETE FROM %s WHERE version > %s", th.SchemaMigrations, p.dialect.GetPlaceholder(1))

	_, err = p.db.Exec(q, target)
	if err != nil {
		return fmt.Errorf("failed to set version to %d: %w", target, err)
	}
	return nil
}

// LoadEnv loads environment variables from a migration run record
func (p *Store) LoadEnv(th TableNames, version int, direction string) (map[string]string, error) {
	q := fmt.Sprintf("SELECT env_json FROM %s WHERE version = %s AND direction = %s ORDER BY id DESC LIMIT 1",
		th.MigrationRuns, p.dialect.GetPlaceholder(1), p.dialect.GetPlaceholder(2))

	var envJSON *string
	err := p.db.QueryRow(q, version, direction).Scan(&envJSON)
	if err == sql.ErrNoRows {
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
func (p *Store) LoadStoredEnv(th TableNames, version int) (map[string]string, error) {
	q := fmt.Sprintf("SELECT name, value FROM %s WHERE version = %s", th.StoredEnv, p.dialect.GetPlaceholder(1))

	rows, err := p.db.Query(q, version)
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
func (p *Store) DeleteStoredEnv(th TableNames, version int) error {
	q := fmt.Sprintf("DELETE FROM %s WHERE version = %s", th.StoredEnv, p.dialect.GetPlaceholder(1))

	_, err := p.db.Exec(q, version)
	if err != nil {
		return fmt.Errorf("failed to delete stored env for version %d: %w", version, err)
	}
	return nil
}

// RecordRun records a migration run with PostgreSQL-specific time handling
func (p *Store) RecordRun(th TableNames, version int, direction string, status int, body *string, env map[string]string, failed bool) error {
	var envJSON *string
	if len(env) > 0 {
		b, err := json.Marshal(env)
		if err != nil {
			return fmt.Errorf("failed to marshal environment for PostgreSQL migration run record (version %d, direction %s): %w", version, direction, err)
		}
		s := string(b)
		envJSON = &s
	}

	ranAt := p.dialect.ConvertTimeToStorage(time.Now().UTC())
	failedVal := p.dialect.ConvertBoolToStorage(failed)

	q := fmt.Sprintf("INSERT INTO %s(version, direction, status_code, body, env_json, failed, ran_at) VALUES(%s,%s,%s,%s,%s,%s,%s)",
		th.MigrationRuns,
		p.dialect.GetPlaceholder(1), p.dialect.GetPlaceholder(2), p.dialect.GetPlaceholder(3),
		p.dialect.GetPlaceholder(4), p.dialect.GetPlaceholder(5), p.dialect.GetPlaceholder(6),
		p.dialect.GetPlaceholder(7))

	_, err := p.db.Exec(q, version, direction, status, body, envJSON, failedVal, ranAt)
	if err != nil {
		return fmt.Errorf("failed to record PostgreSQL migration run (version %d, direction %s, status %d): %w", version, direction, status, err)
	}
	return nil
}

// InsertStoredEnv inserts stored environment variables
func (p *Store) InsertStoredEnv(th TableNames, version int, kv map[string]string) error {
	if len(kv) == 0 {
		return nil
	}

	valuesClauses := make([]string, 0, len(kv))
	args := make([]interface{}, 0, len(kv)*3)
	argIndex := 1

	for name, value := range kv {
		valuesClauses = append(valuesClauses, fmt.Sprintf("(%s,%s,%s)",
			p.dialect.GetPlaceholder(argIndex), p.dialect.GetPlaceholder(argIndex+1), p.dialect.GetPlaceholder(argIndex+2)))
		args = append(args, version, name, value)
		argIndex += 3
	}

	q := fmt.Sprintf("INSERT INTO %s(version, name, value) VALUES %s ON CONFLICT (version, name) DO UPDATE SET value = EXCLUDED.value",
		th.StoredEnv, strings.Join(valuesClauses, ","))

	_, err := p.db.Exec(q, args...)
	if err != nil {
		return fmt.Errorf("failed to insert stored environment for PostgreSQL version %d: %w", version, err)
	}
	return nil
}

// ListRuns returns migration run history with PostgreSQL-specific type handling
func (p *Store) ListRuns(th TableNames) ([]Run, error) {
	q := fmt.Sprintf("SELECT id, version, direction, status_code, body, env_json, failed, ran_at FROM %s ORDER BY id ASC", th.MigrationRuns)

	rows, err := p.db.Query(q)
	if err != nil {
		return nil, fmt.Errorf("failed to list PostgreSQL migration runs: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var runs []Run
	for rows.Next() {
		var run Run
		var body sql.NullString
		var envJSON sql.NullString
		var ranAt time.Time
		var failed bool

		err := rows.Scan(&run.ID, &run.Version, &run.Direction, &run.StatusCode, &body, &envJSON, &failed, &ranAt)
		if err != nil {
			return nil, fmt.Errorf("failed to scan PostgreSQL migration run: %w", err)
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

		run.Failed = failed
		run.RanAt = p.dialect.ConvertTimeFromStorage(&ranAt)

		runs = append(runs, run)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating PostgreSQL migration runs: %w", err)
	}
	return runs, nil
}
