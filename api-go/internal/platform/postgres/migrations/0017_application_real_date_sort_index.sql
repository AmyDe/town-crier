-- +goose Up

-- Backs the SEO authority read's real-date ordering (#819, decision 1): the
-- fix swaps last_different (a PlanIt re-index marker that can bump to "now" on
-- an old application and float it to the top) for
-- GREATEST(decided_date, start_date) DESC NULLS LAST, tie-broken by
-- start_date DESC NULLS LAST then planit_name — the exact column order and
-- direction the query's ORDER BY uses (see recentRealDateOrder in
-- store_postgres.go), so the whole ORDER BY is index-served, not just a leading
-- prefix. GREATEST is a deterministic function of its (immutable) date-column
-- arguments, so it is usable in an expression index.
CREATE INDEX IF NOT EXISTS applications_authority_real_date
    ON applications (
        authority_code,
        (GREATEST(decided_date, start_date)) DESC NULLS LAST,
        start_date DESC NULLS LAST,
        planit_name
    );

-- +goose Down

DROP INDEX IF EXISTS applications_authority_real_date;
