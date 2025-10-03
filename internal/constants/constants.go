package constants

import (
	"net/http"
	"time"
)

// Database Constants
const (
	// PostgreSQL defaults
	DefaultPostgresPort    = 5432
	DefaultPostgresSSLMode = "disable"

	// Connection pool settings
	DefaultPostgresMaxConnections = 25
	DefaultPostgresMaxIdleConns   = 5
	DefaultSQLiteMaxConnections   = 1 // SQLite allows only one writer
	DefaultSQLiteMaxIdleConns     = 1

	// Default table names
	DefaultSchemaMigrationsTable = "schema_migrations"
	DefaultMigrationRunsTable    = "migration_runs"
	DefaultStoredEnvTable        = "stored_env"

	// Table name suffixes when using prefixes
	SchemaMigrationsSuffix = "_schema_migrations"
	MigrationLogSuffix     = "_migration_log"
	StoredEnvSuffix        = "_stored_env"
)

// Time and Duration Constants
const (
	// Connection pool lifetimes
	DefaultMaxConnLifetime = 5 * time.Minute
	DefaultMaxIdleTime     = 1 * time.Minute
	DefaultSQLiteLifetime  = 10 * time.Minute
	DefaultSQLiteIdleTime  = 5 * time.Minute
)

// Wait Configuration Constants
const (
	DefaultWaitTimeout  = 60 * time.Second
	DefaultWaitInterval = 2 * time.Second
	DefaultWaitStatus   = http.StatusOK // Use standard library constant
	DefaultWaitMethod   = "GET"
)
