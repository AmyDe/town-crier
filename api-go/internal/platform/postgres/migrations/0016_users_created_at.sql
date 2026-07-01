-- +goose Up

-- Add a creation timestamp to users so the admin user list can sort oldest-first
-- (issue #729). The column is owned entirely by the database: the DEFAULT
-- CURRENT_TIMESTAMP stamps it on insert and Go never writes it (the shared
-- upsert query is deliberately left untouched). Existing rows get the migration
-- timestamp — not their real sign-up time, which Postgres never captured — which
-- is acceptable: the compound (created_at, user_id) sort has user_id as the
-- essential tiebreak so the backfilled rows still order deterministically.
ALTER TABLE users ADD COLUMN created_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP;

-- +goose Down
ALTER TABLE users DROP COLUMN IF EXISTS created_at;
