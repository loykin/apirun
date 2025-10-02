package store

import (
	"github.com/loykin/apirun/internal/store/connector"
	"github.com/loykin/apirun/internal/store/postgresql"
	"github.com/loykin/apirun/internal/store/sqlite"
)

// Driver constants
const (
	DriverSqlite     = "sqlite"
	DriverPostgresql = "postgresql"
)

type Config struct {
	Driver       string `mapstructure:"driver"`
	TableNames   connector.TableNames
	DriverConfig DriverConfig
}

type DriverConfig interface {
	ToMap() map[string]interface{}
}

// Type aliases for external use - no need to import subpackages directly
type SqliteConfig = sqlite.Config
type PostgresConfig = postgresql.Config
type TableNames = connector.TableNames
