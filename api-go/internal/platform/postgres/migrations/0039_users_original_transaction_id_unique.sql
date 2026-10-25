-- +goose NO TRANSACTION
-- +goose Up

-- CONCURRENTLY cannot run inside a transaction.
CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS users_original_transaction_id_unique
    ON users (original_transaction_id)
    WHERE original_transaction_id IS NOT NULL;

-- +goose Down

DROP INDEX CONCURRENTLY IF EXISTS users_original_transaction_id_unique;
