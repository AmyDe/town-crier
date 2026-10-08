-- +goose Up

CREATE TABLE seo_towns (
    authority_code text NOT NULL,
    slug           text NOT NULL,
    name           text NOT NULL,
    population     integer NOT NULL,
    location       geography(Point, 4326) NOT NULL,
    PRIMARY KEY (authority_code, slug)
);
CREATE INDEX seo_towns_location_gist ON seo_towns USING gist (location);

CREATE TABLE seo_town_assignments (
    authority_code  text NOT NULL,
    planit_name     text NOT NULL,
    town_slug       text NOT NULL,
    last_different  timestamptz NOT NULL,
    PRIMARY KEY (authority_code, planit_name),
    FOREIGN KEY (authority_code, planit_name) REFERENCES applications (authority_code, planit_name) ON DELETE CASCADE
);
CREATE INDEX seo_town_assignments_town ON seo_town_assignments (authority_code, town_slug, last_different DESC);

CREATE TABLE seo_pages (
    path              text PRIMARY KEY,
    kind              text NOT NULL CHECK (kind IN ('hub','towns','authority','town')),
    authority_code    text,
    town_slug         text,
    display_name      text NOT NULL,
    authority_name    text,
    total             integer NOT NULL DEFAULT 0,
    status_breakdown  jsonb NOT NULL DEFAULT '[]',
    children          jsonb NOT NULL DEFAULT '[]',
    neighbours        jsonb NOT NULL DEFAULT '[]',
    lat               double precision,
    lng               double precision,
    sort_order        integer NOT NULL
);

CREATE TABLE seo_redirects (
    path   text PRIMARY KEY,
    target text NOT NULL
);

CREATE TABLE seo_state (
    key   text PRIMARY KEY,
    value text NOT NULL
);

-- +goose Down

DROP TABLE IF EXISTS seo_state;
DROP TABLE IF EXISTS seo_redirects;
DROP TABLE IF EXISTS seo_pages;
DROP TABLE IF EXISTS seo_town_assignments;
DROP TABLE IF EXISTS seo_towns;
