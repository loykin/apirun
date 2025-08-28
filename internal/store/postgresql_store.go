package store

import (
	"database/sql"

	_ "github.com/jackc/pgx/v5/stdlib"
)

// OpenPostgres opens a PostgreSQL-backed store using the given DSN.
// Example DSN: postgres://user:pass@host:5432/dbname
func OpenPostgres(dsn string) (*Store, error) {
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, err
	}
	st := &Store{DB: db, isPostgres: true}
	if err := st.EnsureSchema(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return st, nil
}
