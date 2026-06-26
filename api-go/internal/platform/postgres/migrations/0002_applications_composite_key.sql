-- +goose Up

-- Migration 0001 made planit_name a GLOBAL primary key, which is wrong: a PlanIt
-- case reference is only unique WITHIN an authority (see PlanningApplication.
-- CanonicalUID and the Cosmos model — partition key authorityCode, doc id Name).
-- Two authorities can legitimately share a planit_name; a global PK would let one
-- silently overwrite the other, and the memo 0010 Upsert
-- (INSERT ... ON CONFLICT (authority_code, planit_name)) cannot run without a
-- matching unique constraint. Replace the single-column PK with the composite
-- (authority_code, planit_name). The GiST, authority_recent and app_state indexes
-- are unaffected.
ALTER TABLE applications DROP CONSTRAINT applications_pkey;
ALTER TABLE applications ADD CONSTRAINT applications_pkey PRIMARY KEY (authority_code, planit_name);

-- +goose Down

ALTER TABLE applications DROP CONSTRAINT applications_pkey;
ALTER TABLE applications ADD CONSTRAINT applications_pkey PRIMARY KEY (planit_name);
