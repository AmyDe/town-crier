-- +goose Up

-- device_registrations mirrors api-go/internal/devicetokens/document.go
-- (deviceDocument). The Cosmos container is partitioned by /userId with document
-- id == token, so the natural PK is (user_id, token). registered_at is the
-- timestamp the 180-day purge job ages on, replacing the Cosmos TTL.
CREATE TABLE device_registrations (
    user_id       text        NOT NULL,
    token         text        NOT NULL,
    platform      text        NOT NULL,
    registered_at timestamptz NOT NULL,
    PRIMARY KEY (user_id, token)
);

-- Per-user listing for GDPR export / ListByUser.
CREATE INDEX device_registrations_user ON device_registrations (user_id);
-- Supports the 180-day purge sweep (DELETE WHERE registered_at < cutoff).
CREATE INDEX device_registrations_registered_at ON device_registrations (registered_at);

-- +goose Down

DROP TABLE IF EXISTS device_registrations;
