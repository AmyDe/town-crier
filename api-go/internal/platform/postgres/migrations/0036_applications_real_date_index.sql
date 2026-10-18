-- +goose NO TRANSACTION
-- +goose Up

-- applications.RecentNearPoint bounds GREATEST(decided_date, start_date) by a
-- date window so the planner can AND this index with applications_location_gist
-- and read only recent heap rows. applications_authority_real_date cannot serve
-- that predicate because it leads with authority_code.
--
-- CONCURRENTLY avoids blocking polling writes for the build (see 0028). A failed
-- build can leave an INVALID index that IF NOT EXISTS will not replace: DROP
-- INDEX CONCURRENTLY it, then rerun.
CREATE INDEX CONCURRENTLY IF NOT EXISTS applications_real_date
    ON applications ((GREATEST(decided_date, start_date)));

-- +goose Down

DROP INDEX CONCURRENTLY IF EXISTS applications_real_date;
