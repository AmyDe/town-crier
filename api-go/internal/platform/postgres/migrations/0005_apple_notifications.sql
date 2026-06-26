-- +goose Up

-- apple_notifications records App Store Server Notification UUIDs that have
-- been successfully processed, giving the webhook handler at-most-once
-- semantics (Cosmos -> Postgres migration; memo 0010, epic #645). The
-- natural key is notification_uuid: Apple supplies it on every delivery and
-- guarantees uniqueness per notification. A duplicate delivery is detected
-- with one EXISTS read; an idempotent re-mark is absorbed by ON CONFLICT DO
-- UPDATE — matching CosmosNotificationStore's last-writer-wins UpsertItem.
CREATE TABLE apple_notifications (
    notification_uuid  text PRIMARY KEY,
    processed_at       timestamptz NOT NULL
);

-- +goose Down

DROP TABLE IF EXISTS apple_notifications;
