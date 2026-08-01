-- +goose NO TRANSACTION
-- +goose Up

-- Custom-shape (polygon) watch zones (epic #1031, tc-6he3x). A watch zone is
-- either a circle (the existing location + radius_metres, unchanged) or, when
-- boundary is non-NULL, a hand-drawn polygon; boundary IS NOT NULL is the
-- sole discriminator the store and matching queries key on -- no separate
-- zone-type column. location/radius_metres stay NOT NULL: for a polygon zone
-- the server also writes the polygon's centroid into location and its
-- enclosing radius into radius_metres, so every existing circle-shaped read
-- path (map centring, list rows, boundingBox, GDPR export) keeps working
-- unchanged against a custom-shape zone underneath (watchzones/zone.go,
-- WithBoundary).
--
-- Additive and nullable: no backfill, and no existing row is touched.
ALTER TABLE watch_zones
    ADD COLUMN boundary geography(Polygon, 4326);

-- GiST index serving the ST_Covers polygon-containment branch of the notify
-- hot path (FindZonesContaining, store_postgres.go) -- the polygon
-- counterpart to watch_zones_location_gist's ST_DWithin circle branch.
--
-- CONCURRENTLY (tc-5i07d precedent, migration 0018): plain CREATE INDEX
-- takes a lock that blocks writes to watch_zones for the whole build, and
-- this table is on a live write path (zone create/update/delete) for a
-- product with paying customers. CONCURRENTLY avoids that lock at the cost
-- of a longer build (two table scans) and requires running outside a
-- transaction -- see the NO TRANSACTION directive above, which also means
-- this file's statements are NOT atomic as a group: a failed CONCURRENTLY
-- build can leave behind an INVALID index that IF NOT EXISTS will not
-- replace (see incident notes on tc-5i07d).
CREATE INDEX CONCURRENTLY IF NOT EXISTS watch_zones_boundary_gist ON watch_zones USING gist (boundary);

-- +goose Down

DROP INDEX CONCURRENTLY IF EXISTS watch_zones_boundary_gist;

ALTER TABLE watch_zones
    DROP COLUMN IF EXISTS boundary;
