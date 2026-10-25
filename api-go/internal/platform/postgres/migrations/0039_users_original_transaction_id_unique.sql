-- +goose NO TRANSACTION
-- +goose Up

-- A failed CONCURRENTLY build leaves an INVALID index that IF NOT EXISTS would
-- then skip, so drop it first. With duplicate ids the create fails on every run.
-- +goose StatementBegin
DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM pg_index i
        JOIN pg_class c ON c.oid = i.indexrelid
        WHERE c.relname = 'users_original_transaction_id_unique' AND NOT i.indisvalid
    ) THEN
        DROP INDEX users_original_transaction_id_unique;
    END IF;
END
$$;
-- +goose StatementEnd

-- CONCURRENTLY cannot run inside a transaction.
CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS users_original_transaction_id_unique
    ON users (original_transaction_id)
    WHERE original_transaction_id IS NOT NULL;

-- +goose Down

DROP INDEX CONCURRENTLY IF EXISTS users_original_transaction_id_unique;
