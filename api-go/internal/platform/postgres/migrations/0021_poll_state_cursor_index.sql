-- +goose Up

-- Index-based PlanIt pagination (GH#955 PR A, tc-nlvpz): the poll cursor moves
-- from a page-granular resume (cursor_next_page, 1-based) to a record-granular
-- one (cursor_next_index, 0-based), immune to PlanIt's 1MB response-body
-- truncation — a truncated fetch still advances the index by the records
-- actually received, so a resume never skips or mis-derives a page boundary.
--
-- Additive migration: cursor_next_index is a new, nullable column. Existing
-- rows are backfilled from the old column at the fixed legacy page size (100,
-- the only pg_sz the pre-tc-nlvpz client ever sent). cursor_next_page is kept
-- for one release (deploy-overlap safety: an old-code writer mid-rollout may
-- still write it) with a read-time fallback in the store layer; a follow-up
-- bead drops it next release.
ALTER TABLE poll_state
    ADD COLUMN cursor_next_index integer;

UPDATE poll_state
SET cursor_next_index = (cursor_next_page - 1) * 100
WHERE cursor_next_page IS NOT NULL;

-- +goose Down

ALTER TABLE poll_state
    DROP COLUMN IF EXISTS cursor_next_index;
