-- +goose Up
CREATE TABLE IF NOT EXISTS migration_runs (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  version INTEGER NOT NULL,
  direction TEXT NOT NULL,
  status_code INTEGER NOT NULL,
  body TEXT,
  env_json TEXT,
  ran_at TEXT NOT NULL
);

-- +goose Down
DROP TABLE IF EXISTS migration_runs;
