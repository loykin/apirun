package constants

import "time"

// Database Constants
const (
	// PostgreSQL defaults
	DefaultPostgresPort    = 5432
	DefaultPostgresSSLMode = "disable"

	// SQLite defaults
	DefaultSQLiteMaxConnections = 1 // SQLite allows only one writer

	// PostgreSQL connection pool defaults
	DefaultPostgresMaxConnections = 25

	// Default table names
	DefaultSchemaMigrationsTable = "schema_migrations"
	DefaultMigrationRunsTable    = "migration_runs"
	DefaultStoredEnvTable        = "stored_env"

	// Table name suffixes when using prefixes
	SchemaMigrationsSuffix = "_schema_migrations"
	MigrationLogSuffix     = "_migration_log"
	StoredEnvSuffix        = "_stored_env"
)

const (
	StatusOK = 200
)

// Wait Configuration Constants
const (
	DefaultWaitTimeout  = 60 * time.Second
	DefaultWaitInterval = 2 * time.Second
	DefaultWaitStatus   = StatusOK
	DefaultWaitMethod   = "GET"
)
