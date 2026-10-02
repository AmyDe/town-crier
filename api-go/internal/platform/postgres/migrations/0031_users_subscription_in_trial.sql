-- +goose Up
ALTER TABLE users ADD COLUMN subscription_in_trial boolean NOT NULL DEFAULT false;

-- +goose Down
ALTER TABLE users DROP COLUMN IF EXISTS subscription_in_trial;
