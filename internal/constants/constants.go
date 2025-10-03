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

	// Connection pool settings - optimized for migration workloads
	// PostgreSQL: Higher concurrency for batch operations
	DefaultPostgresMaxConnections = 10 // Reduced from 25 - migrations are typically sequential
	DefaultPostgresMaxIdleConns   = 3  // Reduced from 5 - fewer idle connections needed

	// SQLite: Single writer limitation
	DefaultSQLiteMaxConnections = 1 // SQLite allows only one writer
	DefaultSQLiteMaxIdleConns   = 1 // Single connection reuse

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
	// Connection pool lifetimes - optimized for migration patterns
	// PostgreSQL: Balanced between connection reuse and resource cleanup
	DefaultMaxConnLifetime = 3 * time.Minute  // Reduced from 5min - faster rotation
	DefaultMaxIdleTime     = 30 * time.Second // Reduced from 1min - quicker cleanup

	// SQLite: Longer lifetimes due to single connection and file-based nature
	DefaultSQLiteLifetime = 15 * time.Minute // Increased from 10min - better reuse
	DefaultSQLiteIdleTime = 2 * time.Minute  // Reduced from 5min - balance reuse vs cleanup
)

// Wait Configuration Constants
const (
	DefaultWaitTimeout  = 60 * time.Second
	DefaultWaitInterval = 2 * time.Second
	DefaultWaitStatus   = http.StatusOK // Use standard library constant
	DefaultWaitMethod   = "GET"
)

// HTTP Client Pool Constants - optimized for API migration workloads
const (
	// Connection pooling for HTTP clients
	DefaultHTTPMaxIdleConns        = 10 // Reasonable for concurrent API calls
	DefaultHTTPMaxIdleConnsPerHost = 5  // Balanced per-host connections
	DefaultHTTPMaxConnsPerHost     = 10 // Limit per-host to prevent overwhelming

	// Timeouts optimized for API operations
	DefaultHTTPDialTimeout      = 10 * time.Second // Connection establishment
	DefaultHTTPRequestTimeout   = 30 * time.Second // Individual request timeout
	DefaultHTTPKeepAliveTimeout = 30 * time.Second // Keep-alive for connection reuse
	DefaultHTTPIdleConnTimeout  = 90 * time.Second // Idle connection cleanup

	// TLS handshake timeout
	DefaultHTTPTLSHandshakeTimeout = 10 * time.Second
)
