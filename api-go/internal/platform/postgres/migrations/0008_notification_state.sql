-- +goose Up

-- notification_state mirrors api-go/internal/notificationstate/state.go (State)
-- and the NotificationState Cosmos container (one document per user, document
-- id == partition key == user_id). Each user has at most one watermark row.
--
-- The notificationstate PostgresStore reads the notifications table (same pool)
-- for its UnreadCount cross-read, matching the Cosmos store's cross-container
-- COUNT query.
CREATE TABLE notification_state (
    user_id      text        PRIMARY KEY,
    last_read_at timestamptz NOT NULL,
    version      integer     NOT NULL DEFAULT 1
);

-- +goose Down

DROP TABLE IF EXISTS notification_state;
