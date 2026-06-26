-- +goose Up

-- PostGIS supplies the geography type, the GiST spatial index, and the ST_*
-- functions (ST_DWithin, KNN <->) that the partition-bound Cosmos model cannot
-- serve cross-cuttingly. See docs/memo/0010.
CREATE EXTENSION IF NOT EXISTS postgis;

-- applications mirrors api-go/internal/applications/document.go (applicationDocument).
-- planit_name is the natural key (the PlanIt case reference); location is the
-- spheroidal point the spatial reads index on.
CREATE TABLE applications (
    planit_name     text PRIMARY KEY,
    authority_code  text NOT NULL,
    uid             text NOT NULL,
    area_name       text NOT NULL,
    area_id         integer NOT NULL,
    address         text NOT NULL DEFAULT '',
    postcode        text,
    description     text NOT NULL DEFAULT '',
    app_type        text,
    app_state       text,
    app_size        text,
    start_date      date,
    decided_date    date,
    consulted_date  date,
    location        geography(Point, 4326),
    url             text,
    link            text,
    last_different  timestamptz NOT NULL
);

-- GiST index serves ST_DWithin radius filters and the <-> nearest-N operator.
CREATE INDEX applications_location_gist ON applications USING gist (location);
-- Replaces the Cosmos composite index (/authorityCode ASC, /lastDifferent DESC)
-- backing the per-authority SEO reads.
CREATE INDEX applications_authority_recent ON applications (authority_code, last_different DESC);
-- Supports status-chip counts and app_state filters.
CREATE INDEX applications_app_state ON applications (app_state);

-- watch_zones mirrors api-go/internal/watchzones/document.go (watchZoneDocument).
-- A zone is a circle (centre + radius); the notify path matches an application
-- against every zone whose circle contains the application's point.
CREATE TABLE watch_zones (
    id                     uuid PRIMARY KEY,
    user_id                text NOT NULL,
    name                   text NOT NULL,
    location               geography(Point, 4326) NOT NULL,
    radius_metres          double precision NOT NULL,
    authority_id           integer,
    push_enabled           boolean NOT NULL DEFAULT true,
    email_instant_enabled  boolean NOT NULL DEFAULT false,
    created_at             timestamptz NOT NULL DEFAULT now(),
    UNIQUE (user_id, name)
);

-- GiST index serves the notify-path containment query (ST_DWithin(location, point,
-- radius_metres)) across all users' zones in one index, replacing the Cosmos
-- cross-partition bbox-prune-plus-residual scan.
CREATE INDEX watch_zones_location_gist ON watch_zones USING gist (location);
-- Per-user listing of a user's own zones.
CREATE INDEX watch_zones_user ON watch_zones (user_id);

-- +goose Down

DROP TABLE IF EXISTS watch_zones;
DROP TABLE IF EXISTS applications;
-- The postgis extension is intentionally left installed: it may be shared by
-- other objects and re-creating it on every Up is cheap (IF NOT EXISTS).
