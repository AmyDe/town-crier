-- +goose Up

ALTER TABLE users ADD COLUMN IF NOT EXISTS last_weekly_digest_at timestamptz;

-- +goose Down

ALTER TABLE users DROP COLUMN IF EXISTS last_weekly_digest_at;
