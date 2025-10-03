package main

import (
	"github.com/loykin/apirun"
	"github.com/loykin/apirun/internal/constants"
	"github.com/loykin/apirun/internal/util"
)

// TableNameBuilder handles the construction of table names for the store
type TableNameBuilder struct {
	prefix           string
	schemaMigrations string
	migrationRuns    string
	storedEnv        string
}

// NewTableNameBuilder creates a new table name builder with the given configuration
func NewTableNameBuilder(prefix, schemaMigrations, migrationRuns, storedEnv string) *TableNameBuilder {
	return &TableNameBuilder{
		prefix:           prefix,
		schemaMigrations: schemaMigrations,
		migrationRuns:    migrationRuns,
		storedEnv:        storedEnv,
	}
}

// Build constructs the table names based on the configuration
func (b *TableNameBuilder) Build() apirun.TableNames {
	// Trim all table-related strings at once
	fields := util.TrimSpaceFields(
		b.prefix,
		b.schemaMigrations,
		b.migrationRuns,
		b.storedEnv,
	)
	prefix, sm, mr, se := fields[0], fields[1], fields[2], fields[3]

	// Apply prefix defaults if prefix is provided but specific names are empty
	if prefix != "" {
		if sm == "" {
			sm = prefix + constants.SchemaMigrationsSuffix
		}
		if mr == "" {
			mr = prefix + constants.MigrationLogSuffix
		}
		if se == "" {
			se = prefix + constants.StoredEnvSuffix
		}
	}

	return apirun.TableNames{
		SchemaMigrations: sm,
		MigrationRuns:    mr,
		StoredEnv:        se,
	}
}
