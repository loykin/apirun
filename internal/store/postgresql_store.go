package store

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/loykin/apimigrate/internal/common"
)

const DriverPostgresql = "postgresql"

// PostgreSQL configuration constants
const (
	postgresDefaultPort    = 5432
	postgresDefaultSSLMode = "disable"
)

type PostgresConfig struct {
	DSN      string `mapstructure:"dsn"`
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	User     string `mapstructure:"user"`
	Password string `mapstructure:"password"`
	DBName   string `mapstructure:"dbname"`
	SSLMode  string `mapstructure:"sslmode"`
	dsn      string
}

func (p *PostgresConfig) ToMap() map[string]interface{} {
	// Prefer explicit DSN; otherwise, build from components when host is provided.
	dsn := strings.TrimSpace(p.DSN)
	if dsn == "" && strings.TrimSpace(p.Host) != "" {
		port := p.Port
		if port == 0 {
			port = postgresDefaultPort
		}
		ssl := strings.TrimSpace(p.SSLMode)
		if ssl == "" {
			ssl = postgresDefaultSSLMode
		}
		// Build DSN in the common form accepted by pgx stdlib.
		dsn = fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=%s",
			strings.TrimSpace(p.User), strings.TrimSpace(p.Password),
			strings.TrimSpace(p.Host), port, strings.TrimSpace(p.DBName), ssl,
		)
	}
	p.dsn = dsn
	return map[string]interface{}{
		"dsn": dsn,
	}
}

func NewPostgresConnector() Connector {
	connector := PostgresStore{}
	return &connector
}

type PostgresStore struct {
	DSN string
	db  *sql.DB
}

func (p *PostgresStore) Close() error {
	return p.db.Close()
}

func (p *PostgresStore) Validate() error {
	return nil
}

func (p *PostgresStore) Load(config map[string]interface{}) error {
	if dsn, ok := config["dsn"].(string); ok && dsn != "" {
		p.DSN = dsn
	}
	return nil
}

func (p *PostgresStore) Ensure(th TableNames) error {
	stmts := []string{
		fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s (version INTEGER PRIMARY KEY)", th.SchemaMigrations),
		fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s (id SERIAL PRIMARY KEY, version INTEGER NOT NULL, direction TEXT NOT NULL, status_code INTEGER NOT NULL, body TEXT NULL, env_json TEXT NULL, failed BOOLEAN NOT NULL DEFAULT FALSE, ran_at TIMESTAMPTZ NOT NULL)", th.MigrationRuns),
		fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s (version INTEGER NOT NULL, name TEXT NOT NULL, value TEXT NOT NULL, PRIMARY KEY(version, name))", th.StoredEnv),
	}

	for i, q := range stmts {
		if _, err := p.db.Exec(q); err != nil {
			return fmt.Errorf("failed to create table %d in PostgreSQL schema setup: %w", i+1, err)
		}
	}

	return nil
}

func (p *PostgresStore) Apply(th TableNames, v int) error {
	// #nosec G201 -- table name is validated via Store.safeTableNames (regex), values use placeholders
	q := fmt.Sprintf("INSERT INTO %s(version) VALUES($1) ON CONFLICT DO NOTHING", th.SchemaMigrations)
	_, err := p.db.Exec(q, v)
	if err != nil {
		return fmt.Errorf("failed to apply PostgreSQL migration version %d: %w", v, err)
	}
	return nil
}

func (p *PostgresStore) IsApplied(th TableNames, v int) (bool, error) {
	// #nosec G201 -- table identifier validated by safeTableNames; WHERE uses bind parameter $1
	q := fmt.Sprintf("SELECT 1 FROM %s WHERE version = $1", th.SchemaMigrations)
	row := p.db.QueryRow(q, v)
	var one int
	err := row.Scan(&one)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("failed to check if PostgreSQL migration version %d is applied: %w", v, err)
	}
	return true, nil
}

func (p *PostgresStore) CurrentVersion(th TableNames) (int, error) {
	// #nosec G201 -- sanitized table identifier only; query has no user-controlled parts
	q := fmt.Sprintf("SELECT COALESCE(MAX(version), 0) FROM %s", th.SchemaMigrations)
	row := p.db.QueryRow(q)
	var v int
	if err := row.Scan(&v); err != nil {
		return 0, fmt.Errorf("failed to get current PostgreSQL migration version: %w", err)
	}
	return v, nil
}

func (p *PostgresStore) ListApplied(th TableNames) ([]int, error) {
	// #nosec G201 -- table identifier sanitized prior to use; no user-supplied data in SQL
	q := fmt.Sprintf("SELECT version FROM %s ORDER BY version ASC", th.SchemaMigrations)
	rows, err := p.db.Query(q)
	if err != nil {
		return nil, fmt.Errorf("failed to list applied PostgreSQL migrations: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []int
	for rows.Next() {
		var v int
		if err := rows.Scan(&v); err != nil {
			return nil, fmt.Errorf("failed to scan PostgreSQL migration version from database: %w", err)
		}
		out = append(out, v)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error occurred while iterating over applied PostgreSQL migrations: %w", err)
	}
	return out, nil
}

func (p *PostgresStore) Remove(th TableNames, v int) error {
	// #nosec G201 -- sanitized table identifier; WHERE clause uses parameterized query $1
	q := fmt.Sprintf("DELETE FROM %s WHERE version = $1", th.SchemaMigrations)
	_, err := p.db.Exec(q, v)
	if err != nil {
		return fmt.Errorf("failed to remove PostgreSQL migration version %d: %w", v, err)
	}
	return nil
}

func (p *PostgresStore) SetVersion(th TableNames, target int) error {
	cur, err := p.CurrentVersion(th)
	if err != nil {
		return fmt.Errorf("failed to get current version for PostgreSQL SetVersion operation: %w", err)
	}
	if target == cur {
		return nil
	}
	if target > cur {
		return fmt.Errorf("cannot set PostgreSQL version up from %d to %d; apply migrations instead", cur, target)
	}
	// #nosec G201 -- validated table identifier; comparison value passed as bind parameter $1
	q := fmt.Sprintf("DELETE FROM %s WHERE version > $1", th.SchemaMigrations)
	_, err = p.db.Exec(q, target)
	if err != nil {
		return fmt.Errorf("failed to set PostgreSQL version to %d (was %d): %w", target, cur, err)
	}
	return nil
}

func (p *PostgresStore) RecordRun(th TableNames, version int, direction string, status int, body *string, env map[string]string, failed bool) error {
	var envJSON *string
	if len(env) > 0 {
		b, err := json.Marshal(env)
		if err != nil {
			return fmt.Errorf("failed to marshal environment for PostgreSQL migration run record (version %d, direction %s): %w", version, direction, err)
		}
		s := string(b)
		envJSON = &s
	}
	ranAt := time.Now().UTC()
	// #nosec G201 -- only the table name (validated) is interpolated; all values use bind parameters
	q := fmt.Sprintf("INSERT INTO %s(version, direction, status_code, body, env_json, failed, ran_at) VALUES($1,$2,$3,$4,$5,$6,$7)", th.MigrationRuns)
	_, err := p.db.Exec(q, version, direction, status, body, envJSON, failed, ranAt)
	if err != nil {
		return fmt.Errorf("failed to record PostgreSQL migration run (version %d, direction %s, status %d): %w", version, direction, status, err)
	}
	return nil
}

func (p *PostgresStore) LoadEnv(th TableNames, version int, direction string) (map[string]string, error) {
	// #nosec G201 -- validated table identifier only; predicate values parameterized ($1,$2)
	q := fmt.Sprintf("SELECT env_json FROM %s WHERE version = $1 AND direction = $2 ORDER BY id DESC LIMIT 1", th.MigrationRuns)
	row := p.db.QueryRow(q, version, direction)
	var envJSON sql.NullString
	if err := row.Scan(&envJSON); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return map[string]string{}, nil
		}
		return map[string]string{}, fmt.Errorf("failed to load environment for PostgreSQL migration version %d direction %s: %w", version, direction, err)
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

func (p *PostgresStore) InsertStoredEnv(th TableNames, version int, kv map[string]string) error {
	if len(kv) == 0 {
		return nil
	}
	// #nosec G201 -- sanitized table name; UPSERT uses bind parameters exclusively
	q := fmt.Sprintf("INSERT INTO %s(version,name,value) VALUES($1,$2,$3) ON CONFLICT(version,name) DO UPDATE SET value=EXCLUDED.value", th.StoredEnv)
	for k, v := range kv {
		if _, err := p.db.Exec(q, version, k, v); err != nil {
			return fmt.Errorf("failed to insert stored environment variable %q for PostgreSQL version %d: %w", k, version, err)
		}
	}
	return nil
}

func (p *PostgresStore) LoadStoredEnv(th TableNames, version int) (map[string]string, error) {
	// #nosec G201 -- only validated table name is interpolated; version is a parameter ($1)
	q := fmt.Sprintf("SELECT name, value FROM %s WHERE version = $1", th.StoredEnv)
	rows, err := p.db.Query(q, version)
	if err != nil {
		return map[string]string{}, fmt.Errorf("failed to load stored environment for PostgreSQL version %d: %w", version, err)
	}
	defer func() { _ = rows.Close() }()
	out := map[string]string{}
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			return map[string]string{}, fmt.Errorf("failed to scan stored environment variable for PostgreSQL version %d: %w", version, err)
		}
		out[k] = v
	}
	if err := rows.Err(); err != nil {
		return map[string]string{}, fmt.Errorf("error occurred while reading stored environment for PostgreSQL version %d: %w", version, err)
	}
	return out, nil
}

func (p *PostgresStore) DeleteStoredEnv(th TableNames, version int) error {
	// #nosec G201 -- table identifier from safeTableNames; DELETE predicate uses parameter $1
	q := fmt.Sprintf("DELETE FROM %s WHERE version = $1", th.StoredEnv)
	_, err := p.db.Exec(q, version)
	if err != nil {
		return fmt.Errorf("failed to delete stored environment for PostgreSQL version %d: %w", version, err)
	}
	return nil
}

func (p *PostgresStore) ListRuns(th TableNames) ([]Run, error) {
	// #nosec G201 -- only the sanitized table name is interpolated; values are scanned safely
	q := fmt.Sprintf("SELECT id, version, direction, status_code, body, env_json, failed, ran_at FROM %s ORDER BY id ASC", th.MigrationRuns)
	rows, err := p.db.Query(q)
	if err != nil {
		return nil, fmt.Errorf("failed to query PostgreSQL migration runs: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []Run
	for rows.Next() {
		var (
			id      int
			ver     int
			dir     string
			status  int
			body    sql.NullString
			envJSON sql.NullString
			failed  bool
			ranAt   time.Time
		)
		if err := rows.Scan(&id, &ver, &dir, &status, &body, &envJSON, &failed, &ranAt); err != nil {
			return nil, fmt.Errorf("failed to scan PostgreSQL migration run record: %w", err)
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
		out = append(out, Run{ID: id, Version: ver, Direction: dir, StatusCode: status, Body: bptr, Env: m, Failed: failed, RanAt: ranAt.UTC().Format(time.RFC3339Nano)})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error occurred while reading PostgreSQL migration runs: %w", err)
	}
	return out, nil
}

func (p *PostgresStore) Connect() (*sql.DB, error) {
	logger := common.GetLogger().WithStore("postgresql")
	logger.Debug("connecting to PostgreSQL database", "dsn", p.DSN)

	db, err := sql.Open("pgx", p.DSN)
	if err != nil {
		logger.Error("failed to open PostgreSQL database", err, "dsn", p.DSN)
		return nil, fmt.Errorf("failed to open PostgreSQL database with DSN %q: %w", p.DSN, err)
	}
	p.db = db
	logger.Info("PostgreSQL database connection established successfully")
	return db, nil
}
