-- +goose Up
CREATE TABLE IF NOT EXISTS schema_migrations (
  version INTEGER PRIMARY KEY,
  applied_at TIMESTAMPTZ NOT NULL
);

-- +goose Down
DROP TABLE IF EXISTS schema_migrations;
