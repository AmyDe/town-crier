-- +goose Up

-- users mirrors api-go/internal/profiles/document.go (profileDocument).
-- Partition strategy: in Cosmos the Users container is partitioned by /id (== the
-- Auth0 user id). In Postgres, user_id is the natural primary key — every /v1/me
-- operation is a point read/write, and the admin scans are plain WHERE filters
-- (email, digest_day, last_active_at_epoch, tier).
--
-- version is a monotonic optimistic-lock counter for the watch-zone quota CAS:
-- UpdateZoneCountWithCAS does UPDATE ... WHERE user_id=$1 AND version=$expected
-- and increments version on success. General Save calls preserve the existing
-- version (version is excluded from the ON CONFLICT SET clause).
--
-- zone_preferences stores the per-zone notification channel settings as jsonb,
-- mirroring the map[string]zonePreferencesDocument embedded in profileDocument.
--
-- Nullable columns follow the profileDocument pointer semantics:
--   email_digest_enabled / saved_decision_* — NULL hydrates as true (opt-in
--   default for legacy documents written before these fields existed; see
--   coalesceTrue in document.go).
--   email, original_transaction_id — absent until the field is set.
--   subscription_expiry, grace_period_expiry — absent for Free tier.
--   watch_zone_count — absent for legacy profiles (lazy-init on first quota use).
--
-- last_active_at_epoch is the numeric Unix-millisecond mirror of last_active_at.
-- It is the dormant-scan filter column: the dormant query uses epoch because the
-- timestamp string had two wire formats in Cosmos ("+00:00" and "Z") that do not
-- sort lexicographically. The epoch is always derived from last_active_at.
CREATE TABLE users (
    user_id                     text        PRIMARY KEY,
    email                       text,
    push_enabled                boolean     NOT NULL,
    digest_day                  int         NOT NULL,
    email_digest_enabled        boolean,
    saved_decision_push         boolean,
    saved_decision_email        boolean,
    zone_preferences            jsonb       NOT NULL DEFAULT '{}',
    tier                        text        NOT NULL,
    subscription_expiry         timestamptz,
    original_transaction_id     text,
    grace_period_expiry         timestamptz,
    last_active_at              timestamptz NOT NULL,
    last_active_at_epoch        bigint      NOT NULL,
    watch_zone_count            int,
    version                     int         NOT NULL DEFAULT 0
);

CREATE INDEX users_email                    ON users (email);
CREATE INDEX users_original_transaction_id  ON users (original_transaction_id);
CREATE INDEX users_digest_day               ON users (digest_day);
CREATE INDEX users_last_active_at_epoch     ON users (last_active_at_epoch);
CREATE INDEX users_tier                     ON users (tier);

-- +goose Down

DROP TABLE IF EXISTS users;
