-- +goose Up
CREATE INDEX IF NOT EXISTS idx_stored_env_version ON stored_env(version);

-- +goose Down
DROP INDEX IF EXISTS idx_stored_env_version;
