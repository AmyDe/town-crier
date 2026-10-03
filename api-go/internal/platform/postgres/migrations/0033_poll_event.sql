-- +goose Up

CREATE TABLE poll_event (
    id      bigserial   PRIMARY KEY,
    at      timestamptz NOT NULL,
    kind    text        NOT NULL CHECK (kind IN ('window_short','window_violation','window_missed','surge','oracle_done')),
    axis    text        CHECK (axis IN ('start','decided')),
    day     date,
    detail  int         NOT NULL DEFAULT 0
);
CREATE INDEX poll_event_kind_at ON poll_event (kind, at);

-- +goose Down

DROP TABLE IF EXISTS poll_event;
