-- +goose Up

-- Btree index backing the server-side ?sort=newest / ?sort=oldest keyset scan on
-- the watch-zone applications list (epic #682 slice 1). The newest ORDER BY is
-- (start_date DESC NULLS LAST, authority_code, planit_name); oldest reuses this
-- same index scanned backward. (authority_code, planit_name) is the unique
-- tiebreak — a PlanIt case reference is only unique within an authority — so the
-- keyset cursor never overlaps or gaps on equal start_dates.
--
-- The radius predicate stays served by applications_location_gist (ST_DWithin /
-- KNN <->); this index orders the in-radius candidate set and keeps the keyset
-- pagination cheap. NULLS LAST matches the query so NULL-start_date rows trail
-- the dated rows and still page deterministically.
CREATE INDEX IF NOT EXISTS applications_start_date_keyset
    ON applications (start_date DESC NULLS LAST, authority_code, planit_name);

-- +goose Down

DROP INDEX IF EXISTS applications_start_date_keyset;
