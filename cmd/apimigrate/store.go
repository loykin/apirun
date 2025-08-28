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
		return &apimigrate.StoreOptions{Backend: "postgres", PostgresDSN: dsn}
	}
	// default to sqlite if type is provided but not recognized as postgres
	return &apimigrate.StoreOptions{Backend: "sqlite", SQLitePath: strings.TrimSpace(doc.Store.SQLite.Path)}
}
