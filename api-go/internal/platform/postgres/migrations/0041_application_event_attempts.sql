-- +goose Up

ALTER TABLE application_event ADD COLUMN IF NOT EXISTS attempts integer NOT NULL DEFAULT 0;

-- +goose Down

ALTER TABLE application_event DROP COLUMN IF EXISTS attempts;
