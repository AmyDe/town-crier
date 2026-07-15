-- +goose Up

-- backfill_state is a SINGLETON row (id is always `true`; the CHECK forces at
-- most one row ever) tracking Lane D's national, date-windowed backward
-- sweep (GH#967, ADR 0042): the current window's fixed upper start_date
-- bound (window_end), how far pagination has progressed within it
-- (cursor_next_index), whether the window in progress has produced anything
-- yet (window_records_seen — decides if a fully-drained window counts toward
-- consecutive_empty_windows), and complete, set once the sweep has crept back
-- far enough that it can stop for good. Deliberately a NEW table, not a reuse
-- of poll_state: poll_state's cursor/HWM columns are slated for deletion once
-- the ADR 0041 soak completes, and this lane only ever needs one row, not the
-- per-authority shape poll_state carries.
CREATE TABLE backfill_state (
    id                        boolean     PRIMARY KEY DEFAULT true CHECK (id),
    window_end                date,
    cursor_next_index         integer     NOT NULL DEFAULT 0,
    window_records_seen       integer     NOT NULL DEFAULT 0,
    consecutive_empty_windows integer     NOT NULL DEFAULT 0,
    complete                  boolean     NOT NULL DEFAULT false,
    last_run_time             timestamptz
);

INSERT INTO backfill_state (id) VALUES (true);

-- +goose Down

DROP TABLE IF EXISTS backfill_state;
