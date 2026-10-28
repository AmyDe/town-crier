-- +goose Up

SET LOCAL lock_timeout = '5s';
ALTER TABLE notifications ADD COLUMN IF NOT EXISTS push_status text;
ALTER TABLE notifications ADD COLUMN IF NOT EXISTS push_attempted_at timestamptz;

-- +goose Down

ALTER TABLE notifications DROP COLUMN IF EXISTS push_attempted_at;
ALTER TABLE notifications DROP COLUMN IF EXISTS push_status;
