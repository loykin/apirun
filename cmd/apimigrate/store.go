package main

import (
	"fmt"
	"strings"

	"github.com/loykin/apimigrate"
)

// buildStoreOptionsFromDoc inspects the Store section of the decoded ConfigDoc
// and returns migration.StoreOptions reflecting the requested backend.
//
// Behavior:
// - If type is one of: postgres, postgresql, pg -> choose postgres backend.
//   - If DSN is provided, use it.
//   - Else if Host is provided, build a DSN from components with defaults:
//   - default port 5432
//   - default sslmode "disable"
//   - Otherwise (including "sqlite" or any other non-empty value) -> choose sqlite backend
//     with the provided sqlite.path (may be empty to use default later).
//   - If type is empty, return nil (meaning: use default sqlite in migrator).
func buildStoreOptionsFromDoc(doc ConfigDoc) *apimigrate.StoreOptions {
	stType := strings.ToLower(strings.TrimSpace(doc.Store.Type))
	if stType == "" {
		return nil
	}

	// Derive table names from table_prefix when explicit names are not provided
	prefix := strings.TrimSpace(doc.Store.TablePrefix)
	sm := strings.TrimSpace(doc.Store.TableSchemaMigrations)
	mr := strings.TrimSpace(doc.Store.TableMigrationRuns)
	se := strings.TrimSpace(doc.Store.TableStoredEnv)
	if prefix != "" {
		if sm == "" {
			sm = prefix + "_schema_migrations"
		}
		if mr == "" {
			mr = prefix + "_migration_runs"
		}
		if se == "" {
			se = prefix + "_stored_env"
		}
	}

	if stType == "postgres" || stType == "postgresql" || stType == "pg" {
		dsn := strings.TrimSpace(doc.Store.Postgres.DSN)
		if dsn == "" && strings.TrimSpace(doc.Store.Postgres.Host) != "" {
			port := doc.Store.Postgres.Port
			if port == 0 {
				port = 5432
			}
			ssl := strings.TrimSpace(doc.Store.Postgres.SSLMode)
			if ssl == "" {
				ssl = "disable"
			}
			dsn = fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=%s",
				strings.TrimSpace(doc.Store.Postgres.User), strings.TrimSpace(doc.Store.Postgres.Password),
				strings.TrimSpace(doc.Store.Postgres.Host), port, strings.TrimSpace(doc.Store.Postgres.DBName), ssl,
			)
		}
		return &apimigrate.StoreOptions{
			Backend:                 "postgres",
			PostgresDSN:             dsn,
			TableSchemaMigrations:   sm,
			TableMigrationRuns:      mr,
			TableStoredEnv:          se,
			IndexStoredEnvByVersion: strings.TrimSpace(doc.Store.IndexStoredEnvByVersion),
		}
	}
	// default to sqlite if type is provided but not recognized as postgres
	return &apimigrate.StoreOptions{
		Backend:                 "sqlite",
		SQLitePath:              strings.TrimSpace(doc.Store.SQLite.Path),
		TableSchemaMigrations:   sm,
		TableMigrationRuns:      mr,
		TableStoredEnv:          se,
		IndexStoredEnvByVersion: strings.TrimSpace(doc.Store.IndexStoredEnvByVersion),
	}
}
