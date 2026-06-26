-- +goose Up

-- saved_applications mirrors api-go/internal/savedapplications/document.go
-- (savedApplicationDocument). The Cosmos container is partitioned by /userId with
-- document id "{userId}:{applicationUid}", so the natural PK is
-- (user_id, application_uid). The embedded applications.SnapshotDocument is stored
-- as jsonb so the list endpoint and snapshot refresher need no extra hydration
-- query and nothing the export/refresh needs is lost.
--
-- authority_id is load-bearing on the hot poll fan-out path (UserIDsForApplication):
-- PlanIt uids collide across councils (tc-th98 / GH#384), so the composite index
-- on (application_uid, authority_id) makes the query index-served.
CREATE TABLE saved_applications (
    user_id         text        NOT NULL,
    application_uid text        NOT NULL,
    authority_id    integer     NOT NULL DEFAULT 0,
    saved_at        timestamptz NOT NULL,
    snapshot        jsonb,
    PRIMARY KEY (user_id, application_uid)
);

-- Per-user listing for GDPR export / GetByUserID.
CREATE INDEX saved_applications_user ON saved_applications (user_id);
-- Serves the cross-partition UserIDsForApplication fan-out (hot poll path).
CREATE INDEX saved_applications_application ON saved_applications (application_uid, authority_id);

-- +goose Down

DROP TABLE IF EXISTS saved_applications;
