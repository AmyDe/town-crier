-- +goose Up

-- Multi-redemption offer codes (GH#866): every code now carries an
-- admin-facing label and a redemption cap (max_redemptions, default 1 — the
-- single-use case is just max_redemptions = 1). Individual redemptions move
-- off offer_codes and into a child table, offer_code_redemptions, one row per
-- (code, user) pair, so a code can be redeemed by up to max_redemptions
-- distinct users.
--
-- Additive only: this migration adds columns/constraints/a table and backfills
-- existing rows. It does NOT drop the legacy redeemed / redeemed_by_user_id /
-- redeemed_at columns on offer_codes — RedeemWithCAS and
-- AnonymiseRedemptionsByUserID keep dual-writing them going forward, so a
-- rolled-back binary still recognises a consumed code. Dropping them is an
-- explicit follow-up bead once the new model has soaked.
ALTER TABLE offer_codes
    ADD COLUMN label            text,
    ADD COLUMN max_redemptions  integer NOT NULL DEFAULT 1,
    ADD COLUMN redemption_count integer NOT NULL DEFAULT 0;

ALTER TABLE offer_codes
    ADD CONSTRAINT offer_codes_max_redemptions_range CHECK (max_redemptions BETWEEN 1 AND 10000),
    ADD CONSTRAINT offer_codes_redemption_count_range CHECK (redemption_count BETWEEN 0 AND max_redemptions);

-- offer_code_redemptions is the tombstone-preserving child table: GDPR erasure
-- (AnonymiseRedemptionsByUserID) nulls user_id and redeemed_at on a row but
-- keeps the row itself, so redemption_count (never decremented) and the
-- child-row count stay in lockstep as the consumed-slot invariant.
CREATE TABLE offer_code_redemptions (
    id          bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    code        text NOT NULL REFERENCES offer_codes(code) ON DELETE CASCADE,
    user_id     text,       -- NULL after GDPR anonymisation (row is a tombstone)
    redeemed_at timestamptz -- NULL after GDPR anonymisation
);

-- Enforces "one redemption per user per code". Partial: an anonymised row has
-- a NULL user_id and must not block a future distinct redeemer from reusing
-- that slot's uniqueness check (there is no "future redeemer with a NULL
-- user_id" to collide with).
CREATE UNIQUE INDEX offer_code_redemptions_code_user
    ON offer_code_redemptions (code, user_id) WHERE user_id IS NOT NULL;

-- Serves RedeemedByUserID / RedeemedByUsers (GDPR export, admin active-offer
-- lookup) and AnonymiseRedemptionsByUserID without a full-table scan.
CREATE INDEX offer_code_redemptions_user
    ON offer_code_redemptions (user_id) WHERE user_id IS NOT NULL;

-- Backfill applying the legacy-coalesce rule (store_postgres.go's scanCode
-- comment, pre-dating this migration): a row counts as redeemed if
-- redeemed = true OR redeemed_by_user_id IS NOT NULL — the latter predates the
-- redeemed boolean column. This preserves the redemption_count ==
-- child-row-count invariant for every pre-existing row: fresh (0, no rows),
-- redeemed (1, one child row carrying the redeemer), and already-anonymised
-- (1, one child row too — redeemed is already true so the INSERT below still
-- matches and copies across the already-NULL redeemed_by_user_id/redeemed_at,
-- producing a tombstone row identical to what
-- AnonymiseRedemptionsByUserID would itself have produced).
UPDATE offer_codes
SET redemption_count = 1
WHERE redeemed OR redeemed_by_user_id IS NOT NULL;

INSERT INTO offer_code_redemptions (code, user_id, redeemed_at)
SELECT code, redeemed_by_user_id, redeemed_at
FROM offer_codes
WHERE redeemed OR redeemed_by_user_id IS NOT NULL;

-- +goose Down

DROP TABLE IF EXISTS offer_code_redemptions;

ALTER TABLE offer_codes
    DROP CONSTRAINT IF EXISTS offer_codes_redemption_count_range,
    DROP CONSTRAINT IF EXISTS offer_codes_max_redemptions_range;

ALTER TABLE offer_codes
    DROP COLUMN IF EXISTS redemption_count,
    DROP COLUMN IF EXISTS max_redemptions,
    DROP COLUMN IF EXISTS label;
