-- +goose Up

-- leases holds a single row (id='polling') that gates concurrent poll cycles.
-- A worker acquires the lease by atomically inserting the row (absent case) or
-- replacing it when its expires_at has elapsed (expired case); a live row blocks
-- competing callers. The holder_id and acquired_at are diagnostic metadata only;
-- acquisition decisions compare expires_at and rows-affected.
-- Mirrors the Cosmos leaseDocument shape (internal/polling/leasestore.go).
CREATE TABLE leases (
    id           text        PRIMARY KEY,
    holder_id    text        NOT NULL,
    acquired_at  timestamptz NOT NULL,
    expires_at   timestamptz NOT NULL
);

-- +goose Down

DROP TABLE IF EXISTS leases;
