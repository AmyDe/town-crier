-- +goose NO TRANSACTION
-- +goose Up

-- pg_trgm backs the address fuzzy-match tier of GET /v1/applications/search
-- (#821 Phase 3): the `%` similarity operator and the similarity() ranking
-- function. Allow-listed on Azure Database for PostgreSQL Flexible Server.
CREATE EXTENSION IF NOT EXISTS pg_trgm;

-- Tier 2 (address fuzzy match): the query text is typically much shorter than
-- the full address (a partial/typed-so-far fragment), so matching uses
-- word_similarity (the <% / %> operator family) rather than plain similarity
-- (%) — whole-string similarity would under-score a short, otherwise-exact
-- fragment against a long address. A GIN trigram index on the LOWER-cased
-- address (matching the lower(address) expression the query side uses, for
-- case-insensitive matching) serves <%/%> without a sequential scan. Per
-- pg_trgm convention, the index goes on the argument being searched into
-- (address), not the search term.
--
-- CONCURRENTLY (tc-5i07d): plain CREATE INDEX takes a lock that blocks
-- writes to applications for the whole build — twice fatal in prod, where it
-- blocked the polling worker and then hit the pgmigrate timeout. CONCURRENTLY
-- avoids that lock at the cost of a longer build (two table scans) and
-- requires running outside a transaction — see the NO TRANSACTION directive
-- above, which also means this file's statements are NOT atomic as a group:
-- a failure partway through can leave a later statement unapplied (rerunning
-- the migration is safe/idempotent via IF NOT EXISTS, except that a failed
-- CONCURRENTLY build can leave behind an INVALID index that IF NOT EXISTS
-- will not replace — see incident notes on tc-5i07d).
CREATE INDEX CONCURRENTLY IF NOT EXISTS applications_address_trgm
    ON applications USING gin (lower(address) gin_trgm_ops);

-- Tier 3 (description full-text match): a GIN index over the `english`-config
-- tsvector expression. This is an EXPRESSION index, not a stored generated
-- column, so no change to the Upsert path or appColumns projection is needed —
-- the query side re-derives the identical expression
-- (to_tsvector('english', description)), which is what lets the planner match
-- it to this index.
CREATE INDEX CONCURRENTLY IF NOT EXISTS applications_description_fts
    ON applications USING gin (to_tsvector('english', description));

-- Tier 1 (reference exact/prefix match): uid is the fuller, human-recognisable
-- council planning reference (e.g. "24/0001/FUL") — distinct from planit_name,
-- PlanIt's own shorter internal id (e.g. "24/0001") that the share/by-slug
-- routes key on. uid carries no index today. A case-insensitive functional
-- btree with text_pattern_ops serves both the exact form (lower(uid) = $1) and
-- the prefix form (lower(uid) LIKE $1 || '%') without a sequential scan,
-- regardless of the database's collation.
CREATE INDEX CONCURRENTLY IF NOT EXISTS applications_uid_lower_pattern
    ON applications (lower(uid) text_pattern_ops);

-- +goose Down

DROP INDEX CONCURRENTLY IF EXISTS applications_uid_lower_pattern;
DROP INDEX CONCURRENTLY IF EXISTS applications_description_fts;
DROP INDEX CONCURRENTLY IF EXISTS applications_address_trgm;
-- pg_trgm is intentionally left installed on Down: it may be shared by other
-- objects, and re-creating it on a subsequent Up is cheap (IF NOT EXISTS) —
-- mirroring the postgis extension's Down comment in 0001_init_postgis.sql.
