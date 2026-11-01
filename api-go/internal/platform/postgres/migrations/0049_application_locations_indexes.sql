-- +goose Up

ALTER TABLE application_locations ADD PRIMARY KEY (authority_code, planit_name);
CREATE INDEX application_locations_location ON application_locations USING gist (location);
CREATE INDEX application_locations_geometry ON application_locations USING gist ((location::geometry));
ANALYZE application_locations;

-- +goose Down

DROP INDEX IF EXISTS application_locations_geometry;
DROP INDEX IF EXISTS application_locations_location;
ALTER TABLE application_locations DROP CONSTRAINT IF EXISTS application_locations_pkey;
