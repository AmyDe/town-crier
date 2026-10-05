-- +goose Up

CREATE TABLE planit_call (
    id           bigserial   PRIMARY KEY,
    at           timestamptz NOT NULL,
    work         text        NOT NULL,
    window_day   date,
    page_index   int         NOT NULL,
    status       int,
    total        int,
    retry_after  interval
);
CREATE INDEX planit_call_at ON planit_call (at);

CREATE TABLE poll_window (
    axis               text        NOT NULL CHECK (axis IN ('start','decided')),
    day                date        NOT NULL,
    last_complete_at   timestamptz,
    last_full_read_at  timestamptz,
    last_total         int,
    PRIMARY KEY (axis, day)
);

CREATE TABLE delta_seen (
    axis     text        NOT NULL,
    day      date        NOT NULL,
    uid      text        NOT NULL,
    area_id  int         NOT NULL,
    seen_at  timestamptz NOT NULL,
    PRIMARY KEY (axis, day, uid, area_id)
);

CREATE TABLE application_event (
    id              bigserial   PRIMARY KEY,
    uid             text        NOT NULL,
    authority_code  text        NOT NULL,
    kind            text        NOT NULL CHECK (kind IN ('new_application','decision')),
    event_date      date,
    detected_at     timestamptz NOT NULL DEFAULT now(),
    status          text        NOT NULL DEFAULT 'pending'
                    CHECK (status IN ('pending','sent','stale')),
    processed_at    timestamptz
);
CREATE INDEX application_event_pending ON application_event (detected_at) WHERE status = 'pending';

CREATE TABLE poll_window_member (
    axis     text        NOT NULL,
    day      date        NOT NULL,
    uid      text        NOT NULL,
    area_id  int         NOT NULL,
    read_at  timestamptz NOT NULL,
    PRIMARY KEY (axis, day, uid, area_id)
);

CREATE TABLE poll_oracle_diff (
    axis      text        NOT NULL,
    day       date        NOT NULL,
    uid       text        NOT NULL,
    area_id   int         NOT NULL,
    found_at  timestamptz NOT NULL,
    reason    text,
    PRIMARY KEY (axis, day, uid, area_id)
);

-- +goose Down

DROP TABLE IF EXISTS poll_oracle_diff;
DROP TABLE IF EXISTS poll_window_member;
DROP TABLE IF EXISTS application_event;
DROP TABLE IF EXISTS delta_seen;
DROP TABLE IF EXISTS poll_window;
DROP TABLE IF EXISTS planit_call;
