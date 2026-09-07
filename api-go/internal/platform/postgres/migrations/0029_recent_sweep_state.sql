-- +goose Up

-- recent_sweep_state is a SINGLETON row tracking ADR 0047's Lane E: the
-- looping recent-window start_date sweep that backstops Lanes A/B (GH#1134).
-- The id smallint PRIMARY KEY DEFAULT 1 CHECK (id = 1) forces exactly one row
-- ever.
--
--   lap_anchor            the fixed upper bound of the lap currently in
--                         progress (NULL = the lane has never started). Fixed
--                         for the lap: a floor recomputed from a moving "today"
--                         would recede as the lap runs.
--   window_end            the upper start_date bound of the WindowWidthDays
--                         slice currently draining (slides backward as slices
--                         drain).
--   cursor_next_index     pagination progress within that slice.
--   laps_completed        how many full verification laps have finished.
--   last_lap_completed_at when the last lap finished.
--   last_run_time         the lane's last-run time, for the planner's
--                         laneEIdleInterval pacing gate.
--
-- There is deliberately NO `complete` column: Lane D has one because history
-- runs out, but the recent band never does, so Lane E is a perpetual verifier.
--
-- Its OWN table, NOT a second backfill_state row: backfill_state is keyless and
-- its UPDATE carries no WHERE clause, so a second row there would make every
-- Lane E save silently clobber Lane D's cursor. The explicit id PRIMARY KEY
-- with the CHECK is the deliberate fix for that trap;
-- PostgresRecentSweepStateStore.Save writes `WHERE id = 1`.
CREATE TABLE recent_sweep_state (
    id                    smallint    PRIMARY KEY DEFAULT 1 CHECK (id = 1),
    lap_anchor            date,
    window_end            date,
    cursor_next_index     integer     NOT NULL DEFAULT 0,
    laps_completed        integer     NOT NULL DEFAULT 0,
    last_lap_completed_at timestamptz,
    last_run_time         timestamptz
);

INSERT INTO recent_sweep_state (id) VALUES (1);

-- +goose Down

DROP TABLE IF EXISTS recent_sweep_state;
