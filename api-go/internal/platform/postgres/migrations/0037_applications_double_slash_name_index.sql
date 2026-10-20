-- +goose NO TRANSACTION
-- +goose Up

-- Serves the slash-collapsed fallback in applications.GetByAuthorityAndName. The
-- edge merges runs of slashes in request paths, so a stored name containing "//"
-- arrives collapsed. The index is partial so it only holds the handful of such
-- rows; the query must repeat the LIKE predicate for the planner to use it.
-- CONCURRENTLY avoids blocking polling writes (see 0028).
CREATE INDEX CONCURRENTLY IF NOT EXISTS applications_collapsed_slash_name
    ON applications (authority_code, regexp_replace(planit_name, '/{2,}', '/', 'g'))
    WHERE planit_name LIKE '%//%';

-- +goose Down

DROP INDEX CONCURRENTLY IF EXISTS applications_collapsed_slash_name;
