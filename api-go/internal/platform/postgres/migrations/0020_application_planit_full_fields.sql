-- +goose Up

-- PlanIt full-field widening (GH#935): persist every non-DP-restricted field
-- PlanIt publishes for a planning application, not just the 18 core fields
-- ingested today. All seven columns are additive, nullable, and unindexed —
-- no query path reads them yet (they back the ingester's silent-field
-- comparison and future product features, not any live read today). altid and
-- associated_id are jsonb rather than text because PlanIt's dictionary
-- describes both as plural "identifiers": the wire shape may be a JSON string
-- OR an array depending on the scraper, and jsonb tolerates either without
-- ever failing a record. other_fields holds PlanIt's entire other_fields map
-- verbatim, minus the three DP-restricted keys (applicant_name, agent_name,
-- case_officer) stripped at the application layer before it ever reaches this
-- column.
ALTER TABLE applications
    ADD COLUMN reference      text,
    ADD COLUMN altid          jsonb,
    ADD COLUMN associated_id  jsonb,
    ADD COLUMN last_changed   timestamptz,
    ADD COLUMN last_scraped   timestamptz,
    ADD COLUMN scraper_name   text,
    ADD COLUMN other_fields   jsonb;

-- +goose Down

ALTER TABLE applications
    DROP COLUMN IF EXISTS other_fields,
    DROP COLUMN IF EXISTS scraper_name,
    DROP COLUMN IF EXISTS last_scraped,
    DROP COLUMN IF EXISTS last_changed,
    DROP COLUMN IF EXISTS associated_id,
    DROP COLUMN IF EXISTS altid,
    DROP COLUMN IF EXISTS reference;
