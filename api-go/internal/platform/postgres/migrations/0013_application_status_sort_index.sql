-- +goose Up

-- Btree index backing the server-side ?sort=status keyset scan on the watch-zone
-- applications list (epic #682 slice 2). The status ORDER BY is
-- (app_state ASC NULLS LAST, start_date DESC NULLS LAST, authority_code, planit_name);
-- the index column order, direction, AND nulls-ordering match it exactly so it is
-- usable for both the ordering and the mixed-direction keyset scan.
--
-- start_date DESC is spelled NULLS LAST explicitly: Postgres defaults a DESC column
-- to NULLS FIRST, which would not match the query's NULLS LAST and would leave the
-- index unusable for the keyset. (authority_code, planit_name) is the unique
-- tiebreak — a PlanIt case reference is only unique within an authority — so the
-- keyset cursor never overlaps or gaps on equal (app_state, start_date).
--
-- The radius predicate stays served by applications_location_gist (ST_DWithin /
-- KNN <->); this index orders the in-radius candidate set and keeps the keyset
-- pagination cheap.
CREATE INDEX IF NOT EXISTS applications_app_state_keyset
    ON applications (app_state ASC NULLS LAST, start_date DESC NULLS LAST, authority_code, planit_name);

-- +goose Down

DROP INDEX IF EXISTS applications_app_state_keyset;
