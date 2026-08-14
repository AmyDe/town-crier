-- +goose Up

-- Phase 2 of the watch_zones.authority_id removal (epic tc-9nbs4, GH#1076).
-- Phase 1 (tc-9nbs4.2, PR #1080) removed every Go code path that reads or
-- writes this column -- resolution, storage, and the wire format (the
-- surviving `authorityId` in the API response is a hardcoded-0 back-compat
-- shim for old clients, tc-9nbs4.6, backed by no column). Notification
-- matching is purely geographic (ADR 0041/0044, notifydispatch/decision.go)
-- and has never read this column, so dropping it has no effect on delivery.
--
-- Sequenced as its own PR/migration, opened only after Phase 1 has been
-- live in prod and soaked, specifically so a column-reading old replica can
-- never run against a schema that's already dropped it mid-rollout.
--
-- No backfill needed: the column is nullable with no default
-- (0001_init_postgis.sql:49), so this is a straight drop.
ALTER TABLE watch_zones
    DROP COLUMN authority_id;

-- +goose Down

-- Restores the column shape only, not its data: any authority_id values
-- that existed before the Up ran are gone and cannot be recovered by this
-- Down -- nothing in the running system re-resolves or backfills them.
ALTER TABLE watch_zones
    ADD COLUMN authority_id integer;
