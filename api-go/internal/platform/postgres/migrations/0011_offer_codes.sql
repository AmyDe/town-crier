-- +goose Up

-- offer_codes holds one row per redeemable code granting a paid tier for a
-- fixed duration. code is the canonical 12-character Crockford primary key.
--
-- GDPR Art. 17 tombstone invariant: `redeemed` (boolean) is the authoritative
-- consumed flag. AnonymiseRedemptionsByUserID scrubs redeemed_by_user_id and
-- redeemed_at (PII) but keeps redeemed=true, so an erased code stays consumed
-- and can never be re-redeemed. The RedeemWithCAS predicate MUST be
-- `redeemed = false` — NOT `redeemed_by_user_id IS NULL` — because an
-- anonymised code has redeemed=true with a NULL redeemed_by_user_id.
--
-- Mirrors the Cosmos offerCodeDocument shape (internal/offercodes/document.go).
CREATE TABLE offer_codes (
    code                 text        PRIMARY KEY,
    tier                 text        NOT NULL,
    duration_days        integer     NOT NULL,
    created_at           timestamptz NOT NULL,
    redeemed             boolean     NOT NULL DEFAULT false,
    redeemed_by_user_id  text,
    redeemed_at          timestamptz
);

-- Serves RedeemedByUserID (GDPR export) and AnonymiseRedemptionsByUserID
-- without a full-table scan. Partial: only rows with a redeemer need the index
-- (unredeemed and anonymised codes have NULL redeemed_by_user_id).
CREATE INDEX offer_codes_redeemed_by ON offer_codes (redeemed_by_user_id)
    WHERE redeemed_by_user_id IS NOT NULL;

-- +goose Down

DROP TABLE IF EXISTS offer_codes;
