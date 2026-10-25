-- +goose Up

ALTER TABLE planit_call ADD COLUMN IF NOT EXISTS outcome text;

-- +goose Down

ALTER TABLE planit_call DROP COLUMN IF EXISTS outcome;
