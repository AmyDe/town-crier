-- +goose Up
CREATE TABLE IF NOT EXISTS webhook_endpoints (
    id                   uuid        PRIMARY KEY,
    user_id              text        NOT NULL UNIQUE,
    url                  text        NOT NULL,
    format               text        NOT NULL CHECK (format IN ('json', 'slack')),
    secret               text        NOT NULL,
    status               text        NOT NULL CHECK (status IN ('active', 'disabled')),
    disabled_reason      text        NULL CHECK (disabled_reason IN ('failures', 'gone')),
    consecutive_failures int         NOT NULL DEFAULT 0,
    last_success_at      timestamptz NULL,
    last_failure_at      timestamptz NULL,
    last_failure_reason  text        NULL,
    last_failure_status  int         NULL,
    last_test_at         timestamptz NULL,
    created_at           timestamptz NOT NULL,
    updated_at           timestamptz NOT NULL
);

CREATE TABLE IF NOT EXISTS webhook_deliveries (
    id              uuid        PRIMARY KEY,
    endpoint_id     uuid        NOT NULL REFERENCES webhook_endpoints (id) ON DELETE CASCADE,
    notification_id text        NOT NULL,
    event_type      text        NOT NULL,
    payload         jsonb       NOT NULL,
    status          text        NOT NULL CHECK (status IN ('pending', 'delivered', 'failed')),
    attempts        int         NOT NULL DEFAULT 0,
    created_at      timestamptz NOT NULL,
    last_attempt_at timestamptz NULL,
    delivered_at    timestamptz NULL,
    UNIQUE (endpoint_id, notification_id)
);

CREATE INDEX IF NOT EXISTS webhook_deliveries_pending_idx
    ON webhook_deliveries (endpoint_id, created_at) WHERE status = 'pending';

-- +goose Down
DROP TABLE IF EXISTS webhook_deliveries;
DROP TABLE IF EXISTS webhook_endpoints;
