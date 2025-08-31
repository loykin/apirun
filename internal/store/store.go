package store

import (
	"database/sql"
	"errors"
	"fmt"
	"regexp"
	"strings"

	_ "github.com/jackc/pgx/v5/stdlib"
	_ "modernc.org/sqlite"
)

// DbFileName is the default filename for the migration history database.
const DbFileName = "apimigrate.db"

type Store struct {
	DB         *sql.DB
	isPostgres bool
	tn         tableNames
	Driver     string
	connector  Connector
}

// Connect selects a connector based on Driver, loads config, connects, assigns DB/connector
// and ensures schema. It also sets backend flags for placeholder handling.
func (s *Store) Connect(config Config) error {
	var connector Connector
	switch s.Driver {
	case "sqlite":
		connector = NewSqliteConnector()
		_ = connector.Load(config.ToMap())
		// mark backend
		s.isPostgres = false
	case "postgres":
		connector = NewPostgresConnector()
		_ = connector.Load(config.ToMap())
		s.isPostgres = true
	default:
		return errors.New("unknown store driver: " + s.Driver)
	}
	db, err := connector.Connect()
	if err != nil {
		return err
	}
	s.DB = db
	s.connector = connector
	// ensure schema via connector
	if err := s.EnsureSchema(); err != nil {
		_ = s.Close()
		return err
	}
	return nil
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

// SetTableNames allows overriding default table/index names (validated via safeTableNames at use time).
func (s *Store) SetTableNames(schema, runs, env, idx string) {
	s.tn = tableNames{schemaMigrations: schema, migrationRuns: runs, storedEnv: env, idxStoredEnvVersion: idx}
}

// EnsureSchema creates required tables for migration state.
func (s *Store) EnsureSchema() error {
	tn := s.safeTableNames()

	err := s.connector.Ensure(tn)
	if err != nil {
		return err
	}

	return nil
}

func (s *Store) Close() error {
	if s == nil {
		return nil
	}
	if s.connector != nil {
		return s.connector.Close()
	}
	if s.DB != nil {
		return s.DB.Close()
	}
	return nil
}

// Apply records a version as applied (idempotent).
func (s *Store) Apply(v int) error {
	return s.connector.Apply(s.safeTableNames(), v)
}

// conv replaces '?' placeholders with $1, $2... for Postgres; pass-through for SQLite.
func (s *Store) conv(q string) string {
	if !s.isPostgres {
		return q
	}
	n := 0
	var b strings.Builder
	for i := 0; i < len(q); i++ {
		if q[i] == '?' {
			n++
			b.WriteByte('$')
			b.WriteString(fmt.Sprintf("%d", n))
			continue
		}
		b.WriteByte(q[i])
	}
	return b.String()
}

func (s *Store) IsApplied(v int) (bool, error) {
	return s.connector.IsApplied(s.safeTableNames(), v)
}

func (s *Store) CurrentVersion() (int, error) {
	return s.connector.CurrentVersion(s.safeTableNames())
}

func (s *Store) ListApplied() ([]int, error) {
	return s.connector.ListApplied(s.safeTableNames())
}

func (s *Store) Remove(v int) error {
	return s.connector.Remove(s.safeTableNames(), v)
}

func (s *Store) SetVersion(target int) error {
	return s.connector.SetVersion(s.safeTableNames(), target)
}

func (s *Store) RecordRun(version int, direction string, status int, body *string, env map[string]string) error {
	return s.connector.RecordRun(s.safeTableNames(), version, direction, status, body, env)
}

func (s *Store) LoadEnv(version int, direction string) (map[string]string, error) {
	return s.connector.LoadEnv(s.safeTableNames(), version, direction)
}

func (s *Store) InsertStoredEnv(version int, kv map[string]string) error {
	return s.connector.InsertStoredEnv(s.safeTableNames(), version, kv)
}

func (s *Store) LoadStoredEnv(version int) (map[string]string, error) {
	return s.connector.LoadStoredEnv(s.safeTableNames(), version)
}

func (s *Store) DeleteStoredEnv(version int) error {
	return s.connector.DeleteStoredEnv(s.safeTableNames(), version)
}
