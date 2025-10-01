package connector

import "database/sql"

// Run represents a single execution record from the migration_runs table.
// Body may be nil when not saved; Env may be empty when not recorded.
type Run struct {
	ID         int
	Version    int
	Direction  string
	StatusCode int
	Body       *string
	Env        map[string]string
	Failed     bool
	RanAt      string // RFC3339Nano for sqlite; Postgres converted to RFC3339Nano
}

// TableNames represents database table names
type TableNames struct {
	SchemaMigrations string
	MigrationRuns    string
	StoredEnv        string
}

type Connector interface {
	Connect() (*sql.DB, error)
	Validate() error
	Load(config map[string]interface{}) error
	Ensure(th TableNames) error
	Apply(th TableNames, v int) error
	IsApplied(th TableNames, v int) (bool, error)
	CurrentVersion(th TableNames) (int, error)
	ListApplied(th TableNames) ([]int, error)
	Remove(th TableNames, v int) error
	SetVersion(th TableNames, target int) error
	RecordRun(th TableNames, version int, direction string, status int, body *string, env map[string]string, failed bool) error
	LoadEnv(th TableNames, version int, direction string) (map[string]string, error)
	InsertStoredEnv(th TableNames, version int, kv map[string]string) error
	LoadStoredEnv(th TableNames, version int) (map[string]string, error)
	DeleteStoredEnv(th TableNames, version int) error
	// ListRuns returns migration run history ordered by id ASC
	ListRuns(th TableNames) ([]Run, error)
	Close() error
}
