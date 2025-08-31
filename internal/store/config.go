package store

import (
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"strings"
)

type Config struct {
	Driver       string `mapstructure:"driver"`
	TableNames   TableNames
	DriverConfig DriverConfig
}

type DriverConfig interface {
	ToMap() map[string]interface{}
}

// Open opens a SQLite-backed store at the given path and ensures schema.
func Open(path string) (*Store, error) {
	if strings.TrimSpace(path) == "" {
		return nil, errors.New("empty sqlite path")
	}
	// Ensure the parent directory exists to avoid SQLITE_CANTOPEN on create
	dir := filepath.Dir(filepath.Clean(path))
	if err := ensureDir(dir); err != nil {
		return nil, err
	}
	dsn := "file:" + filepath.Clean(path) + "?_busy_timeout=5000&_fk=1"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	st := &Store{Driver: DriverSqlite}
	st.DB = db
	st.connector = &SqliteStore{db: db}
	st.TableName = defaultTableNames()
	if err := st.EnsureSchema(); err != nil {
		_ = st.connector.Close()
		return nil, err
	}
	return st, nil
}

// ensureDir creates the directory if it doesn't exist (noop if it does).
func ensureDir(d string) error {
	if d == "." || strings.TrimSpace(d) == "" {
		return nil
	}
	return os.MkdirAll(d, 0750)
}

// OpenPostgres opens a Postgres-backed store using pgx stdlib DSN and ensures schema.
func OpenPostgres(dsn string) (*Store, error) {
	if strings.TrimSpace(dsn) == "" {
		return nil, errors.New("empty postgres dsn")
	}
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, err
	}
	st := &Store{Driver: DriverPostgresql}
	st.DB = db
	st.connector = &PostgresStore{db: db}
	st.TableName = defaultTableNames()
	if err := st.EnsureSchema(); err != nil {
		_ = st.connector.Close()
		return nil, err
	}
	return st, nil
}
