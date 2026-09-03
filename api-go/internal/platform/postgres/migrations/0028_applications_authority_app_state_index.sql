-- +goose NO TRANSACTION
-- +goose Up

-- applications.BreakdownByAuthority (internal/applications/store_postgres.go)
-- runs
--     SELECT app_state, count(*) FROM applications
--     WHERE authority_code = $1 GROUP BY app_state
-- to build the status-chip counts for the build-key-gated SEO authority read
-- (GET /v1/authorities/{id}/applications, recentHandler.recentByAuthority). It
-- had no covering index. The closest existing index,
-- applications_authority_real_date (0017_application_real_date_sort_index.sql),
-- leads with authority_code so it can prune the authority's rows — but it does
-- NOT carry app_state, so the planner index-scans that authority's slice and
-- then does a heap fetch PER ROW just to read app_state before the aggregate.
-- The only app_state index, applications_app_state (0001_init_postgis.sql), is
-- keyed on app_state alone and cannot prune by authority_code. So the count has
-- always cost one heap fetch per application in the authority, and that cost
-- scales with how many applications the authority has ingested.
--
-- This is the confirmed (tc-bnxbn; App Insights on log-town-crier-shared,
-- 2026-09-03, not assumption) root cause of the SEO Refresh prod job timing
-- out (tc-a6mrb): GET /v1/authorities/{id}/applications ran p50 670ms but
-- p95 ~20s and max ~55s, and the fat tail — amplified by the concurrency-4
-- fetch burst against the single 0.25 vCPU / 0.5 GiB prod API replica and
-- free-tier Postgres — pushed the web-prod fetch step past its ceiling. The
-- sibling RecentByAuthority read (LIMIT 200) is already index-served by 0017
-- and is not the problem.
--
-- A composite btree index on (authority_code, app_state) carries both columns
-- the query touches, so the WHERE + GROUP BY count becomes an index-only scan
-- of the authority's prefix with no heap fetch, independent of authority size.
--
-- CONCURRENTLY (tc-5i07d precedent, see 0018_application_search_indexes.sql and
-- 0027_applications_authority_uid_index.sql): a plain CREATE INDEX takes a lock
-- that blocks writes to applications for the whole build — in prod that blocks
-- the polling worker and can then hit the pgmigrate timeout. CONCURRENTLY
-- avoids that lock at the cost of a longer build (two table scans) and requires
-- running outside a transaction — hence the NO TRANSACTION directive above,
-- which also means this file's statements are NOT atomic as a group. Rerunning
-- the migration is safe/idempotent via IF NOT EXISTS, except that a failed
-- CONCURRENTLY build can leave behind an INVALID index that IF NOT EXISTS will
-- not replace — see the incident notes on tc-5i07d for the recovery recipe
-- (DROP INDEX CONCURRENTLY the invalid index, then rerun).
CREATE INDEX CONCURRENTLY IF NOT EXISTS applications_authority_app_state
    ON applications (authority_code, app_state);

-- +goose Down

DROP INDEX CONCURRENTLY IF EXISTS applications_authority_app_state;
