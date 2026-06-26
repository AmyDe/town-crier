-- +goose Up

-- poll_state holds one row per authority: the wall-clock time of the last poll
-- attempt (last_poll_time, drives LRU scheduling), the PlanIt high-water mark
-- used as the different_start cursor on the next fetch (high_water_mark), and
-- an optional resumable pagination cursor (cursor_* columns). All three cursor
-- fields move as a set; absent cursor_different_start / cursor_next_page means
-- no active cursor. Mirrors internal/polling/state.go (PollState + PollCursor)
-- and the Cosmos pollStateDocument shape.
CREATE TABLE poll_state (
    authority_id            integer     PRIMARY KEY,
    last_poll_time          timestamptz NOT NULL,
    high_water_mark         timestamptz NOT NULL,
    cursor_different_start  date,
    cursor_next_page        integer,
    cursor_known_total      integer
);

-- Index on last_poll_time for the LRU scan: GetLeastRecentlyPolled LEFT JOINs
-- the candidate set from unnest($1) and orders by last_poll_time ASC NULLS FIRST
-- (never-polled rows sort first because their last_poll_time is NULL).
CREATE INDEX poll_state_last_poll_time ON poll_state (last_poll_time);

-- +goose Down

DROP TABLE IF EXISTS poll_state;
