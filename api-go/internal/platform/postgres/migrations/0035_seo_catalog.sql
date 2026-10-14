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

CREATE TABLE seo_town_applications (
    path            text NOT NULL,
    rank            integer NOT NULL,
    authority_code  text NOT NULL,
    planit_name     text NOT NULL,
    PRIMARY KEY (path, rank)
);

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
    lastmod           timestamptz,
    sort_order        integer NOT NULL
);

CREATE TABLE seo_redirects (
    path   text PRIMARY KEY,
    target text NOT NULL
);

-- +goose Down

DROP TABLE IF EXISTS seo_redirects;
DROP TABLE IF EXISTS seo_pages;
DROP TABLE IF EXISTS seo_town_applications;
DROP TABLE IF EXISTS seo_towns;
