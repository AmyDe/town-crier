-- +goose NO TRANSACTION
-- +goose Up

-- Pre-canned keyword filters on watch zones (Pro-tier entitlement, GH#1090,
-- epic tc-w825j). A watch zone may nominate one catalog filterKey (see
-- watchzones.FilterKey, watchzones/filter.go) that gates notification
-- fan-out to applications matching that filter's criteria; the zero value
-- (NULL / empty string) means unfiltered, today's behaviour.
--
-- Additive and nullable: no backfill, no existing row is touched. No index:
-- matching happens per-application in the Enqueuer against an
-- already-loaded zone (notifydispatch/enqueuer.go), not via a
-- `WHERE filter_key = ...` query -- nothing queries watch_zones by
-- filter_key. No CHECK constraint restricting values to the catalog:
-- validity is enforced by the domain/HTTP layer against the Go-code
-- catalog (watchzones.IsValidFilterKey) so the catalog can be revised
-- without a migration.
ALTER TABLE watch_zones ADD COLUMN filter_key text;

-- +goose Down
ALTER TABLE watch_zones DROP COLUMN IF EXISTS filter_key;
