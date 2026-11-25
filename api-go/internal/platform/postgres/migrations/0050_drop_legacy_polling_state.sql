-- +goose Up
DROP TABLE IF EXISTS poll_state;
DROP TABLE IF EXISTS backfill_state;
DROP TABLE IF EXISTS recent_sweep_state;

-- +goose Down
SELECT 1;
