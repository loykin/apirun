package store

import "database/sql"

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
	Close() error
}
