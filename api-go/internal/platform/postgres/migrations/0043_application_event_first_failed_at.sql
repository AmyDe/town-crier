-- +goose Up

ALTER TABLE application_event ADD COLUMN IF NOT EXISTS first_failed_at timestamptz;

-- +goose Down

ALTER TABLE application_event DROP COLUMN IF EXISTS first_failed_at;
