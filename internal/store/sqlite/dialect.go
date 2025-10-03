package sqlite

import (
	"database/sql"
	"fmt"
	"time"
)

// Dialect implements SQL dialect for SQLite
type Dialect struct{}

// NewDialect creates a new SQLite dialect
func NewDialect() *Dialect {
	return &Dialect{}
}

// GetPlaceholder returns SQLite-style placeholders (?)
func (s *Dialect) GetPlaceholder() string {
	return "?"
}

// GetUpsertClause returns SQLite's conflict resolution clause
func (s *Dialect) GetUpsertClause() string {
	return "OR IGNORE"
}

// ConvertBoolToStorage converts bool to SQLite storage format (integer 0/1)
func (s *Dialect) ConvertBoolToStorage(b bool) interface{} {
	if b {
		return 1
	}
	return 0
}

// ConvertTimeToStorage converts time to SQLite storage format (RFC3339Nano string)
func (s *Dialect) ConvertTimeToStorage(t time.Time) interface{} {
	return t.Format(time.RFC3339Nano)
}

// ConvertBoolFromStorage converts SQLite integer storage to bool
func (s *Dialect) ConvertBoolFromStorage(val interface{}) bool {
	if i, ok := val.(int64); ok {
		return i != 0
	}
	if i, ok := val.(int); ok {
		return i != 0
	}
	return false
}

// ConvertTimeFromStorage converts SQLite string storage to RFC3339Nano string
func (s *Dialect) ConvertTimeFromStorage(val interface{}) string {
	if str, ok := val.(string); ok {
		return str
	}
	return ""
}

// Connect establishes a connection to SQLite with connection pooling
func (s *Dialect) Connect(dsn string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open SQLite connection: %w", err)
	}
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("failed to ping SQLite database: %w", err)
	}

	// SQLite-specific configuration (SQLite doesn't support multiple writers)
	db.SetMaxOpenConns(1)                   // SQLite allows only one writer
	db.SetMaxIdleConns(1)                   // Keep one idle connection
	db.SetConnMaxLifetime(10 * time.Minute) // Longer lifetime for SQLite
	db.SetConnMaxIdleTime(5 * time.Minute)  // Longer idle time for SQLite

	return db, nil
}

// GetEnsureStatements returns SQLite-specific table creation statements
func (s *Dialect) GetEnsureStatements(schemaMigrations, migrationRuns, storedEnv string) []string {
	return []string{
		fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s (version INTEGER PRIMARY KEY)", schemaMigrations),
		fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s (id INTEGER PRIMARY KEY AUTOINCREMENT, version INTEGER NOT NULL, direction TEXT NOT NULL, status_code INTEGER NOT NULL, body TEXT NULL, env_json TEXT NULL, failed INTEGER NOT NULL DEFAULT 0, ran_at TEXT NOT NULL)", migrationRuns),
		fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s (version INTEGER NOT NULL, name TEXT NOT NULL, value TEXT NOT NULL, PRIMARY KEY(version, name))", storedEnv),
	}
}

// GetDriverName returns the driver name for logging
func (s *Dialect) GetDriverName() string {
	return "sqlite"
}
