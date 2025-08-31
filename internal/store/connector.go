package store

import "database/sql"

type Connector interface {
	Connect() (*sql.DB, error)
	Validate() error
	Load(config map[string]interface{}) error
	Ensure(th tableNames) error
	Apply(th tableNames, v int) error
	IsApplied(th tableNames, v int) (bool, error)
	CurrentVersion(th tableNames) (int, error)
	ListApplied(th tableNames) ([]int, error)
	Remove(th tableNames, v int) error
	SetVersion(th tableNames, target int) error
	RecordRun(th tableNames, version int, direction string, status int, body *string, env map[string]string) error
	LoadEnv(th tableNames, version int, direction string) (map[string]string, error)
	InsertStoredEnv(th tableNames, version int, kv map[string]string) error
	LoadStoredEnv(th tableNames, version int) (map[string]string, error)
	DeleteStoredEnv(th tableNames, version int) error
	Close() error
}
