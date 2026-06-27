-- +goose Up

-- Btree index backing the recent-activity sort's per-user latest-unread subquery
-- (epic #682 slice 3). That subquery is:
--
--   SELECT application_uid, authority_id, MAX(created_at)
--   FROM notifications n JOIN notification_state ns ON ns.user_id = n.user_id
--   WHERE n.user_id = $1 AND n.created_at > ns.last_read_at
--   GROUP BY application_uid, authority_id
--
-- Migration 0007 already indexes notifications (user_id, created_at) — that serves
-- the user_id equality + created_at range filter, but it has no application_uid, so
-- the per-application GROUP BY + MAX(created_at) still needs a sort/aggregate over
-- the filtered set. This composite leads with user_id (the equality key), then
-- application_uid (the group key), then created_at DESC (so MAX per app is the
-- first entry in each group), letting Postgres satisfy the per-app newest-unread
-- reduction from the index. authority_id is the secondary group key but is
-- effectively functionally dependent on application_uid (a PlanIt uid belongs to
-- one authority), so it is left out to keep the index narrow.
CREATE INDEX IF NOT EXISTS notifications_user_app_created
    ON notifications (user_id, application_uid, created_at DESC);

-- +goose Down

DROP INDEX IF EXISTS notifications_user_app_created;
