package postgresql

import (
	"database/sql"
	"fmt"
	"time"
)

// Adapter implements DatabaseAdapter for PostgreSQL
type Adapter struct{}

// NewAdapter creates a new PostgreSQL adapter
func NewAdapter() *Adapter {
	return &Adapter{}
}

// GetPlaceholder returns PostgreSQL-style placeholders ($1, $2, etc.)
func (p *Adapter) GetPlaceholder(index int) string {
	return fmt.Sprintf("$%d", index)
}

// GetUpsertClause returns PostgreSQL's conflict resolution clause
func (p *Adapter) GetUpsertClause() string {
	return ""
}

// ConvertBoolToStorage converts bool to PostgreSQL storage format (native bool)
func (p *Adapter) ConvertBoolToStorage(b bool) interface{} {
	return b
}

// ConvertTimeToStorage converts time to PostgreSQL storage format (native time.Time)
func (p *Adapter) ConvertTimeToStorage(t time.Time) interface{} {
	return t
}

// ConvertBoolFromStorage converts PostgreSQL bool storage to bool
func (p *Adapter) ConvertBoolFromStorage(val interface{}) bool {
	if b, ok := val.(bool); ok {
		return b
	}
	return false
}

// ConvertTimeFromStorage converts PostgreSQL time storage to RFC3339Nano string
func (p *Adapter) ConvertTimeFromStorage(val interface{}) string {
	if t, ok := val.(*time.Time); ok && t != nil {
		return t.UTC().Format(time.RFC3339Nano)
	}
	if t, ok := val.(time.Time); ok {
		return t.UTC().Format(time.RFC3339Nano)
	}
	return ""
}

// Connect establishes a connection to PostgreSQL
func (p *Adapter) Connect(dsn string) (*sql.DB, error) {
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open PostgreSQL connection: %w", err)
	}
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("failed to ping PostgreSQL database: %w", err)
	}
	return db, nil
}

// GetEnsureStatements returns PostgreSQL-specific table creation statements
func (p *Adapter) GetEnsureStatements(schemaMigrations, migrationRuns, storedEnv string) []string {
	return []string{
		fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s (version INTEGER PRIMARY KEY)", schemaMigrations),
		fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s (id SERIAL PRIMARY KEY, version INTEGER NOT NULL, direction TEXT NOT NULL, status_code INTEGER NOT NULL, body TEXT NULL, env_json TEXT NULL, failed BOOLEAN NOT NULL DEFAULT FALSE, ran_at TIMESTAMPTZ NOT NULL)", migrationRuns),
		fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s (version INTEGER NOT NULL, name TEXT NOT NULL, value TEXT NOT NULL, PRIMARY KEY(version, name))", storedEnv),
	}
}

// GetDriverName returns the driver name for logging
func (p *Adapter) GetDriverName() string {
	return "postgresql"
}
