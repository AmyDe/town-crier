-- +goose NO TRANSACTION
-- +goose Up

-- applications.GetByUID (internal/applications/store_postgres.go) runs
--     SELECT ... FROM applications WHERE authority_code = $1 AND uid = $2
--     ORDER BY planit_name LIMIT 1
-- on every single application ingest (the national delta lanes and the Lane D
-- historical backfill both call it via Ingester.Ingest, for the silent-field
-- read-back HasSameSilentFieldsAs check). The table's only key is the
-- PRIMARY KEY on planit_name (0001_init_postgis.sql); the only index that
-- touches authority_code, applications_authority_recent
-- (authority_code, last_different DESC), doesn't help an equality uid lookup,
-- and the only uid-related index, applications_uid_lower_pattern
-- (0018_application_search_indexes.sql), is a FUNCTIONAL index on
-- lower(uid) text_pattern_ops built for GH#821 case-insensitive search — the
-- planner cannot use it for this query's plain uid = $2 predicate. So
-- GetByUID has always run as an authority-scoped sequential scan, with cost
-- proportional to how many rows exist for that authority.
--
-- This is confirmed (tc-v6f4m; Azure Monitor + App Insights, not assumption)
-- to be the root cause of alert-job-failed-poll-prod paging nightly: overnight
-- the live delta lanes finish fast and cede almost the whole ~9-10 minute
-- poll-cycle budget to Lane D, which walks deep into old historical PlanIt
-- records for large/old authorities. Each ingested record triggers a
-- GetByUID scan against a large, often cold, per-authority row set; enough of
-- those inside one Lane D page (up to pg_sz=300) accumulate to blow the hard
-- cycle deadline mid-query, surfacing as *pgconn.errTimeout /
-- "context deadline exceeded".
--
-- A composite btree index on (authority_code, uid) turns GetByUID into a
-- direct index lookup, independent of how deep Lane D has backfilled.
--
-- CONCURRENTLY (tc-5i07d precedent, see 0018_application_search_indexes.sql):
-- a plain CREATE INDEX takes a lock that blocks writes to applications for
-- the whole build — in prod that previously blocked the polling worker and
-- then hit the pgmigrate timeout. CONCURRENTLY avoids that lock at the cost
-- of a longer build (two table scans) and requires running outside a
-- transaction — hence the NO TRANSACTION directive above, which also means
-- this file's statements are NOT atomic as a group. Rerunning the migration
-- is safe/idempotent via IF NOT EXISTS, except that a failed CONCURRENTLY
-- build can leave behind an INVALID index that IF NOT EXISTS will not
-- replace — see the incident notes on tc-5i07d for the recovery recipe
-- (DROP INDEX CONCURRENTLY the invalid index, then rerun).
CREATE INDEX CONCURRENTLY IF NOT EXISTS applications_authority_uid
    ON applications (authority_code, uid);

-- +goose Down

DROP INDEX CONCURRENTLY IF EXISTS applications_authority_uid;
