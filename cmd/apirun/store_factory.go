package main

import (
	"github.com/loykin/apirun"
	"github.com/loykin/apirun/internal/util"
)

// StoreFactory handles the creation of store configurations using appropriate builders
type StoreFactory struct{}

// NewStoreFactory creates a new store factory
func NewStoreFactory() *StoreFactory {
	return &StoreFactory{}
}

// CreateStoreConfig creates a store configuration from the given config
func (f *StoreFactory) CreateStoreConfig(config StoreConfig) *apirun.StoreConfig {
	if config.Disabled {
		return nil
	}

	stType := util.TrimAndLower(config.Type)
	if stType == "" {
		return nil
	}

	// Build table names using dedicated builder
	tableNameBuilder := NewTableNameBuilder(
		config.TablePrefix,
		config.TableSchemaMigrations,
		config.TableMigrationRuns,
		config.TableStoredEnv,
	)
	tableNames := tableNameBuilder.Build()

	// Use appropriate database-specific builder
	if stType == apirun.DriverPostgresql {
		builder := NewPostgresStoreBuilder(config.Postgres)
		return builder.Build(tableNames)
	}

	// Default to SQLite
	builder := NewSqliteStoreBuilder(config.SQLite.Path)
	return builder.Build(tableNames)
}
