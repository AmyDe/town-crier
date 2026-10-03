-- +goose Up

CREATE TABLE poll_control (
    id          boolean     PRIMARY KEY DEFAULT true CHECK (id),
    enabled     boolean     NOT NULL,
    reason      text        NOT NULL DEFAULT '',
    updated_at  timestamptz NOT NULL
);

-- +goose Down

DROP TABLE IF EXISTS poll_control;
