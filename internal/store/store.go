package store

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

// DbFileName is the default filename for the migration history database.
const DbFileName = "apimigrate.db"

// Store persists migration versions in a SQLite database.
// Table schema_migrations(version INTEGER PRIMARY KEY, applied_at TEXT)
// Similar to goose semantics: version is the numeric prefix of the migration filename.
//
// DB path is a SQLite file path. Use Open(dbPath) to create/connect.

type Store struct {
	DB *sql.DB
}

func Open(dbPath string) (*Store, error) {
	dsn := fmt.Sprintf("file:%s?_busy_timeout=5000&_fk=1", dbPath)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	st := &Store{DB: db}
	if err := st.EnsureSchema(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return st, nil
}

func (s *Store) Close() error {
	if s == nil || s.DB == nil {
		return nil
	}
	return s.DB.Close()
}

func (s *Store) EnsureSchema() error {
	_, err := s.DB.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (
		version INTEGER PRIMARY KEY,
		applied_at TEXT NOT NULL
	)`)
	return err
}

// Apply records that a migration version has been applied.
func (s *Store) Apply(version int) error {
	_, err := s.DB.Exec(`INSERT OR IGNORE INTO schema_migrations(version, applied_at) VALUES(?, ?)`, version, time.Now().UTC().Format(time.RFC3339))
	return err
}

// Remove deletes a migration version record (used for down).
func (s *Store) Remove(version int) error {
	_, err := s.DB.Exec(`DELETE FROM schema_migrations WHERE version = ?`, version)
	return err
}

// IsApplied returns true if the version exists in the table.
func (s *Store) IsApplied(version int) (bool, error) {
	row := s.DB.QueryRow(`SELECT 1 FROM schema_migrations WHERE version = ?`, version)
	var one int
	err := row.Scan(&one)
	if err == sql.ErrNoRows {
		return false, nil
	}
	return err == nil, err
}

// CurrentVersion returns the highest applied version, or 0 if none.
func (s *Store) CurrentVersion() (int, error) {
	row := s.DB.QueryRow(`SELECT COALESCE(MAX(version), 0) FROM schema_migrations`)
	var v int
	if err := row.Scan(&v); err != nil {
		return 0, err
	}
	return v, nil
}

// ListApplied returns applied versions sorted ascending.
func (s *Store) ListApplied() ([]int, error) {
	rows, err := s.DB.Query(`SELECT version FROM schema_migrations ORDER BY version ASC`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []int
	for rows.Next() {
		var v int
		if err := rows.Scan(&v); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

// SetVersion sets the store to an exact version by applying/removing without executing migrations.
// This is dangerous and intended only for edge cases; prefer MigrateUp/MigrateDown flows.
func (s *Store) SetVersion(target int) error {
	cur, err := s.CurrentVersion()
	if err != nil {
		return err
	}
	if target == cur {
		return nil
	}
	if target > cur {
		return errors.New("SetVersion cannot move up; use MigrateUp")
	}
	// moving down: remove all versions > target
	_, err = s.DB.Exec(`DELETE FROM schema_migrations WHERE version > ?`, target)
	return err
}
