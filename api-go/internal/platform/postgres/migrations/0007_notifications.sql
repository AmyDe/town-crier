-- +goose Up

-- notifications mirrors api-go/internal/notifications/digest.go (DigestNotification)
-- and the Notifications Cosmos container (partition key /userId, TTL 90 days).
-- The TTL is replaced by a scheduled PurgeOlderThan(cutoff) DELETE on Postgres.
--
-- Primary key: the notification id (a UUID string minted by the poll fan-out).
-- Uniqueness on (user_id, application_uid, authority_id, event_type) enforces the
-- GetByUserAndApplication dedup invariant at the DB level — the poll fan-out can
-- never insert two notifications for the same (user, app, authority, event).
CREATE TABLE notifications (
    id                      text        PRIMARY KEY,
    user_id                 text        NOT NULL,
    application_uid         text        NOT NULL,
    application_name        text        NOT NULL DEFAULT '',
    watch_zone_id           text,
    application_address     text        NOT NULL DEFAULT '',
    application_description text        NOT NULL DEFAULT '',
    application_type        text,
    authority_id            integer     NOT NULL,
    decision                text,
    event_type              text        NOT NULL,
    sources                 text        NOT NULL DEFAULT '',
    push_sent               boolean     NOT NULL DEFAULT false,
    email_sent              boolean     NOT NULL DEFAULT false,
    created_at              timestamptz NOT NULL,
    UNIQUE (user_id, application_uid, authority_id, event_type)
);

-- (user_id, created_at) backs ByUserSince, AllByUser, and the per-user part of
-- GetLatestUnreadByApplications (WHERE user_id=$1 AND created_at > $3).
CREATE INDEX notifications_user_created ON notifications (user_id, created_at);

-- application_uid alone backs the ANY($2) application set filter inside
-- GetLatestUnreadByApplications.
CREATE INDEX notifications_application_uid ON notifications (application_uid);

-- created_at alone backs the PurgeOlderThan full-table sweep
-- (DELETE WHERE created_at < $1), avoiding a sequential scan at 90-day TTL runs.
CREATE INDEX notifications_created_at ON notifications (created_at);

-- (user_id, email_sent) backs UnsentEmailsByUser (WHERE user_id=$1 AND email_sent=false)
-- and the outer DISTINCT user_id scan in UserIDsWithUnsentEmails.
CREATE INDEX notifications_user_email_sent ON notifications (user_id, email_sent);

-- +goose Down

DROP TABLE IF EXISTS notifications;
