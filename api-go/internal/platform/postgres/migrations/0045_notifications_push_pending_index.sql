-- +goose NO TRANSACTION
-- +goose Up

-- A failed CONCURRENTLY build leaves an INVALID index that IF NOT EXISTS would
-- then skip, so drop it first.
-- +goose StatementBegin
DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM pg_index i
        JOIN pg_class c ON c.oid = i.indexrelid
        WHERE c.relname = 'notifications_push_pending' AND NOT i.indisvalid
    ) THEN
        DROP INDEX notifications_push_pending;
    END IF;
END
$$;
-- +goose StatementEnd

-- CONCURRENTLY cannot run inside a transaction.
CREATE INDEX CONCURRENTLY IF NOT EXISTS notifications_push_pending
    ON notifications (created_at)
    WHERE push_status = 'pending';

-- +goose Down

DROP INDEX CONCURRENTLY IF EXISTS notifications_push_pending;
