package main

import (
	"strings"

	"github.com/loykin/apirun"
)

// SqliteStoreBuilder handles SQLite-specific store configuration
type SqliteStoreBuilder struct {
	path string
}

// NewSqliteStoreBuilder creates a new SQLite store builder
func NewSqliteStoreBuilder(path string) *SqliteStoreBuilder {
	return &SqliteStoreBuilder{path: path}
}

// Build creates a configured SQLite store config with the given table names
func (b *SqliteStoreBuilder) Build(tableNames apirun.TableNames) *apirun.StoreConfig {
	sqlite := &apirun.SqliteConfig{Path: strings.TrimSpace(b.path)}
	return apirun.NewSqliteStoreConfig(sqlite, tableNames)
}
