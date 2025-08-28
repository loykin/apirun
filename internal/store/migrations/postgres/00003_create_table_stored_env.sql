-- +goose Up
CREATE TABLE IF NOT EXISTS stored_env (
  id BIGSERIAL PRIMARY KEY,
  version INTEGER NOT NULL,
  name TEXT NOT NULL,
  value TEXT NOT NULL
);

-- +goose Down
DROP TABLE IF EXISTS stored_env;
