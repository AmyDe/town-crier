-- +goose Up

-- Carries a national lane's descending-walk head across RunOnePage's
-- per-page checkpoints (ADR 0044 resumable model; GH#983). A multi-page
-- walk's true maximum LastDifferent is always the first record of the
-- walk's first page -- but RunOnePage's own maxIngested is scoped to ONE
-- page, so a walk that resumes across cycles was setting the watermark to
-- the BOUNDARY (oldest) page's max instead of the whole walk's head.
-- cursor_walk_head captures that first-page value and threads it through
-- every resume until the walk completes.
--
-- Additive, nullable migration: no backfill. A NULL/absent value on a
-- pre-migration in-flight row is a supported degrade case, not an error --
-- the store layer falls back to the existing per-page maxIngested
-- behaviour when cursor_walk_head is unset (see store_postgres.go).
ALTER TABLE poll_state
    ADD COLUMN cursor_walk_head timestamptz;

-- +goose Down

ALTER TABLE poll_state
    DROP COLUMN IF EXISTS cursor_walk_head;
