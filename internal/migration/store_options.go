package migration

// StoreOptions controls which backend and connection settings the migrator should use.
// It is carried via context to avoid breaking the public API.
type StoreOptions struct {
	Backend     string // "sqlite" (default) or "postgres"
	SQLitePath  string // optional explicit sqlite file path; default is migrate_dir/apimigrate.db
	PostgresDSN string // full DSN for PostgreSQL

	// Optional custom table/index names to allow using multiple sets side-by-side.
	TableSchemaMigrations   string // default: schema_migrations
	TableMigrationRuns      string // default: migration_runs
	TableStoredEnv          string // default: stored_env
	IndexStoredEnvByVersion string // default: idx_stored_env_version
}
