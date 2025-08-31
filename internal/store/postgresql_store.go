package store

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
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
			port = 5432
		}
		ssl := strings.TrimSpace(p.SSLMode)
		if ssl == "" {
			ssl = "disable"
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

func (p *PostgresStore) Ensure(th tableNames) error {
	stmts := []string{
		fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s (version INTEGER PRIMARY KEY)", th.schemaMigrations),
		fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s (id SERIAL PRIMARY KEY, version INTEGER NOT NULL, direction TEXT NOT NULL, status_code INTEGER NOT NULL, body TEXT NULL, env_json TEXT NULL, ran_at TIMESTAMPTZ NOT NULL)", th.migrationRuns),
		fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s (version INTEGER NOT NULL, name TEXT NOT NULL, value TEXT NOT NULL, PRIMARY KEY(version, name))", th.storedEnv),
		fmt.Sprintf("CREATE INDEX IF NOT EXISTS %s ON %s(version)", th.idxStoredEnvVersion, th.storedEnv),
	}

	for _, q := range stmts {
		if _, err := p.db.Exec(q); err != nil {
			return err
		}
	}

	return nil
}

func (p *PostgresStore) Apply(th tableNames, v int) error {
	// #nosec G201 -- only sanitized table name is interpolated; value is a bind parameter
	q := fmt.Sprintf("INSERT INTO %s(version) VALUES($1) ON CONFLICT (version) DO NOTHING", th.schemaMigrations)
	_, err := p.db.Exec(q, v)
	return err
}

func (p *PostgresStore) IsApplied(th tableNames, v int) (bool, error) {
	// #nosec G201 -- table identifier validated by safeTableNames; WHERE uses bind parameter $1
	q := fmt.Sprintf("SELECT 1 FROM %s WHERE version = $1", th.schemaMigrations)
	row := p.db.QueryRow(q, v)
	var one int
	err := row.Scan(&one)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	return err == nil, err
}

func (p *PostgresStore) CurrentVersion(th tableNames) (int, error) {
	// #nosec G201 -- sanitized table identifier only; query has no user-controlled parts
	q := fmt.Sprintf("SELECT COALESCE(MAX(version), 0) FROM %s", th.schemaMigrations)
	row := p.db.QueryRow(q)
	var v int
	if err := row.Scan(&v); err != nil {
		return 0, err
	}
	return v, nil
}

func (p *PostgresStore) ListApplied(th tableNames) ([]int, error) {
	// #nosec G201 -- table identifier sanitized prior to use; no user-supplied data in SQL
	q := fmt.Sprintf("SELECT version FROM %s ORDER BY version ASC", th.schemaMigrations)
	rows, err := p.db.Query(q)
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

func (p *PostgresStore) Remove(th tableNames, v int) error {
	// #nosec G201 -- table name is a validated identifier; predicate uses bind parameter $1
	q := fmt.Sprintf("DELETE FROM %s WHERE version = $1", th.schemaMigrations)
	_, err := p.db.Exec(q, v)
	return err
}

func (p *PostgresStore) SetVersion(th tableNames, target int) error {
	cur, err := p.CurrentVersion(th)
	if err != nil {
		return err
	}
	if target == cur {
		return nil
	}
	if target > cur {
		return errors.New("cannot set version up; apply migrations instead")
	}
	// #nosec G201 -- validated table identifier; comparison value passed as bind parameter $1
	q := fmt.Sprintf("DELETE FROM %s WHERE version > $1", th.schemaMigrations)
	_, err = p.db.Exec(q, target)
	return err
}

func (p *PostgresStore) RecordRun(th tableNames, version int, direction string, status int, body *string, env map[string]string) error {
	var envJSON *string
	if len(env) > 0 {
		b, _ := json.Marshal(env)
		s := string(b)
		envJSON = &s
	}
	ranAt := time.Now().UTC()
	// #nosec G201 -- only the table name (validated) is interpolated; all values use bind parameters
	q := fmt.Sprintf("INSERT INTO %s(version, direction, status_code, body, env_json, ran_at) VALUES($1,$2,$3,$4,$5,$6)", th.migrationRuns)
	_, err := p.db.Exec(q, version, direction, status, body, envJSON, ranAt)
	return err
}

func (p *PostgresStore) LoadEnv(th tableNames, version int, direction string) (map[string]string, error) {
	// #nosec G201 -- validated table identifier only; predicate values parameterized ($1,$2)
	q := fmt.Sprintf("SELECT env_json FROM %s WHERE version = $1 AND direction = $2 ORDER BY id DESC LIMIT 1", th.migrationRuns)
	row := p.db.QueryRow(q, version, direction)
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

func (p *PostgresStore) InsertStoredEnv(th tableNames, version int, kv map[string]string) error {
	if len(kv) == 0 {
		return nil
	}
	// #nosec G201 -- sanitized table name; UPSERT uses bind parameters exclusively
	q := fmt.Sprintf("INSERT INTO %s(version,name,value) VALUES($1,$2,$3) ON CONFLICT(version,name) DO UPDATE SET value=EXCLUDED.value", th.storedEnv)
	for k, v := range kv {
		if _, err := p.db.Exec(q, version, k, v); err != nil {
			return err
		}
	}
	return nil
}

func (p *PostgresStore) LoadStoredEnv(th tableNames, version int) (map[string]string, error) {
	// #nosec G201 -- only validated table name is interpolated; version is a parameter ($1)
	q := fmt.Sprintf("SELECT name, value FROM %s WHERE version = $1", th.storedEnv)
	rows, err := p.db.Query(q, version)
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

func (p *PostgresStore) DeleteStoredEnv(th tableNames, version int) error {
	// #nosec G201 -- table identifier from safeTableNames; DELETE predicate uses parameter $1
	q := fmt.Sprintf("DELETE FROM %s WHERE version = $1", th.storedEnv)
	_, err := p.db.Exec(q, version)
	return err
}

func (p *PostgresStore) Connect() (*sql.DB, error) {
	db, err := sql.Open("pgx", p.DSN)
	if err != nil {
		return nil, err
	}
	p.db = db
	return db, nil
}
