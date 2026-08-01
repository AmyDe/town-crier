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
CREATE INDEX watch_zones_boundary_gist ON watch_zones USING gist (boundary);

-- +goose Down

DROP INDEX IF EXISTS watch_zones_boundary_gist;

ALTER TABLE watch_zones
    DROP COLUMN IF EXISTS boundary;
