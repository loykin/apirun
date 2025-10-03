package postgresql

import (
	"database/sql"
	"fmt"
	"time"
)

// Dialect implements SQL dialect for PostgreSQL
type Dialect struct{}

// NewDialect creates a new PostgreSQL dialect
func NewDialect() *Dialect {
	return &Dialect{}
}

// GetPlaceholder returns PostgreSQL-style placeholders ($1, $2, etc.)
func (p *Dialect) GetPlaceholder(index int) string {
	return fmt.Sprintf("$%d", index)
}

// GetUpsertClause returns PostgreSQL's conflict resolution clause
func (p *Dialect) GetUpsertClause() string {
	return ""
}

// ConvertBoolToStorage converts bool to PostgreSQL storage format (native bool)
func (p *Dialect) ConvertBoolToStorage(b bool) interface{} {
	return b
}

// ConvertTimeToStorage converts time to PostgreSQL storage format (native time.Time)
func (p *Dialect) ConvertTimeToStorage(t time.Time) interface{} {
	return t
}

// ConvertBoolFromStorage converts PostgreSQL bool storage to bool
func (p *Dialect) ConvertBoolFromStorage(val interface{}) bool {
	if b, ok := val.(bool); ok {
		return b
	}
	return false
}

// ConvertTimeFromStorage converts PostgreSQL time storage to RFC3339Nano string
func (p *Dialect) ConvertTimeFromStorage(val interface{}) string {
	if t, ok := val.(*time.Time); ok && t != nil {
		return t.UTC().Format(time.RFC3339Nano)
	}
	if t, ok := val.(time.Time); ok {
		return t.UTC().Format(time.RFC3339Nano)
	}
	return ""
}

// Connect establishes a connection to PostgreSQL with connection pooling
func (p *Dialect) Connect(dsn string) (*sql.DB, error) {
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open PostgreSQL connection: %w", err)
	}

	// Configure connection pool settings
	db.SetMaxOpenConns(25)                 // Maximum number of open connections
	db.SetMaxIdleConns(5)                  // Maximum number of idle connections
	db.SetConnMaxLifetime(5 * time.Minute) // Maximum amount of time a connection may be reused
	db.SetConnMaxIdleTime(1 * time.Minute) // Maximum amount of time a connection may be idle

	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("failed to ping PostgreSQL database: %w", err)
	}
	return db, nil
}

// GetEnsureStatements returns PostgreSQL-specific table creation statements
func (p *Dialect) GetEnsureStatements(schemaMigrations, migrationRuns, storedEnv string) []string {
	return []string{
		fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s (version INTEGER PRIMARY KEY)", schemaMigrations),
		fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s (id SERIAL PRIMARY KEY, version INTEGER NOT NULL, direction TEXT NOT NULL, status_code INTEGER NOT NULL, body TEXT NULL, env_json TEXT NULL, failed BOOLEAN NOT NULL DEFAULT FALSE, ran_at TIMESTAMPTZ NOT NULL)", migrationRuns),
		fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s (version INTEGER NOT NULL, name TEXT NOT NULL, value TEXT NOT NULL, PRIMARY KEY(version, name))", storedEnv),
	}
}

// GetDriverName returns the driver name for logging
func (p *Dialect) GetDriverName() string {
	return "postgresql"
}
