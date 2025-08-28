package store

import (
	"database/sql"
	"embed"
	"fmt"
	"strings"

	"github.com/pressly/goose/v3"
)

// Embed all SQL migrations for both backends
//
//go:embed migrations/**/*.sql
var migrationFS embed.FS

var gooseTableName = "apimigrate_goose_version"

// InitMigrations initializes the embedded migrations
func InitMigrations() {
	goose.SetBaseFS(migrationFS)
	goose.SetTableName(gooseTableName)
}

// runMigrations applies embedded migrations for the given dialect ("sqlite" or "postgres").
func runMigrations(db *sql.DB, dialect string) error {
	// Ensure goose sees our embedded FS and custom table name
	goose.SetBaseFS(migrationFS)
	goose.SetTableName(gooseTableName)
	var dir string
	switch strings.ToLower(strings.TrimSpace(dialect)) {
	case "postgres", "pg", "postgresql":
		err := goose.SetDialect("postgres")
		if err != nil {
			return err
		}
		dir = "migrations/postgres"
	case "sqlite", "sqlite3", "":
		err := goose.SetDialect("sqlite3")
		if err != nil {
			return err
		}
		dir = "migrations/sqlite"
	default:
		return fmt.Errorf("unsupported dialect for migrations: %s", dialect)
	}
	return goose.Up(db, dir)
}
