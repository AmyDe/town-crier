-- +goose Up

-- Per-application read state (issue #733, step 1 of 3). Adds a nullable read_at
-- to notifications so opening an application can mark only that application's
-- notifications read for the user (tap-to-read), replacing the single per-user
-- watermark (notification_state.last_read_at). This migration is deployed ALONE
-- and is deliberately additive/back-compatible: the running API ignores read_at
-- and keeps computing unread from the watermark, so nothing breaks until the API
-- flips to read_at in step 2. The two backfills below make the read_at-based
-- unread set identical to the watermark-based one at flip time.
ALTER TABLE notifications ADD COLUMN read_at timestamptz NULL;

-- Watermarked users: everything at or before the watermark is already read, so
-- stamp read_at with the watermark. Inclusive (<=) mirrors the watermark rule
-- where created_at == last_read_at counts as read.
UPDATE notifications n
SET read_at = ns.last_read_at
FROM notification_state ns
WHERE n.user_id = ns.user_id
  AND n.created_at <= ns.last_read_at
  AND n.read_at IS NULL;

-- Users with no watermark row read as all-read today (the old GET seeded a
-- first-touch watermark of "now" on first access, so their history was never
-- unread). Mark their existing notifications read at their own created_at. Disjoint
-- from the first UPDATE: this only touches users absent from notification_state.
UPDATE notifications n
SET read_at = n.created_at
WHERE n.read_at IS NULL
  AND NOT EXISTS (SELECT 1 FROM notification_state ns WHERE ns.user_id = n.user_id);

-- Partial index over the unread set only. Leading user_id (equality) then
-- application_uid (the per-application mark-read/GROUP BY key) then created_at,
-- with the WHERE read_at IS NULL predicate keeping the index tiny and self-pruning
-- as rows are marked read. Backs the step-2 unread-count, latest-unread-per-app,
-- and zonepage unread-filter queries.
CREATE INDEX IF NOT EXISTS idx_notifications_unread
    ON notifications (user_id, application_uid, created_at)
    WHERE read_at IS NULL;

-- +goose Down

-- The backfill is not reversible (the pre-migration all-NULL read_at state is not
-- recoverable), which is fine for a Down: dropping the column discards read_at
-- entirely and the watermark in notification_state remains authoritative.
DROP INDEX IF EXISTS idx_notifications_unread;
ALTER TABLE notifications DROP COLUMN IF EXISTS read_at;
