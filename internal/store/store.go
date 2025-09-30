package store

import (
	"database/sql"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/loykin/apirun/internal/store/postgresql"
	"github.com/loykin/apirun/internal/store/sqlite"

	_ "github.com/jackc/pgx/v5/stdlib"
	_ "modernc.org/sqlite"
)

// DbFileName is the default filename for the migration history database.
const DbFileName = "apirun.db"

type Store struct {
	DB        *sql.DB
	TableName TableNames
	Driver    string
	connector Connector
}

// Connect selects a connector based on Driver, loads config, connects, assigns DB/connector
// and ensures schema. It also sets backend flags for placeholder handling.
func (s *Store) Connect(config Config) error {
	var connector Connector
	switch config.Driver {
	case DriverSqlite:
		store := sqlite.NewStore()
		if config.DriverConfig != nil {
			_ = store.Load(config.DriverConfig.ToMap())
		}
		connector = &sqliteConnectorWrapper{store: store}
		s.Driver = DriverSqlite
	case DriverPostgresql:
		store := postgresql.NewStore()
		if config.DriverConfig != nil {
			_ = store.Load(config.DriverConfig.ToMap())
		}
		connector = &postgresConnectorWrapper{store: store}
		s.Driver = DriverPostgresql
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

type TableNames struct {
	SchemaMigrations string
	MigrationRuns    string
	StoredEnv        string
}

var identRe = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`)

// safeTableNames returns validated table/index names; if a custom name is invalid,
// it falls back to the default for that identifier to avoid SQL injection via identifiers.
func (s *Store) safeTableNames() TableNames {
	d := defaultTableNames()
	// Prefer externally visible TableName when any field is non-empty; else use internal tn
	t := s.TableName
	// if TableName has any custom non-empty values, start from it
	if s.TableName.SchemaMigrations != "" || s.TableName.MigrationRuns != "" || s.TableName.StoredEnv != "" {
		t = s.TableName
	}
	if !identRe.MatchString(t.SchemaMigrations) || t.SchemaMigrations == "" {
		t.SchemaMigrations = d.SchemaMigrations
	}
	if !identRe.MatchString(t.MigrationRuns) || t.MigrationRuns == "" {
		t.MigrationRuns = d.MigrationRuns
	}
	if !identRe.MatchString(t.StoredEnv) || t.StoredEnv == "" {
		t.StoredEnv = d.StoredEnv
	}
	return t
}

func defaultTableNames() TableNames {
	return TableNames{
		SchemaMigrations: "schema_migrations",
		MigrationRuns:    "migration_runs",
		StoredEnv:        "stored_env",
	}
}

// SetTableNames allows overriding default table names (validated via safeTableNames at use time).
func (s *Store) SetTableNames(schema, runs, env string) {
	t := TableNames{SchemaMigrations: schema, MigrationRuns: runs, StoredEnv: env}
	s.TableName = t
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
	if s.Driver != DriverPostgresql {
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

func (s *Store) RecordRun(version int, direction string, status int, body *string, env map[string]string, failed bool) error {
	return s.connector.RecordRun(s.safeTableNames(), version, direction, status, body, env, failed)
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

// ListRuns returns the migration_runs history records.
func (s *Store) ListRuns() ([]Run, error) {
	return s.connector.ListRuns(s.safeTableNames())
}
