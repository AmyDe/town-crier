package offercodes

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/AmyDe/town-crier/api-go/internal/profiles"
)

// querier is the consumer-side slice of *pgxpool.Pool the Postgres offer-code
// store uses. Both *pgxpool.Pool and pgx.Tx satisfy it structurally, so the
// store is decoupled from the concrete pool type.
type querier interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// PostgresStore reads and writes offer codes in the Postgres `offer_codes` table
// (Cosmos → Postgres migration; memo 0010, epic #645).
//
// Partition strategy: offer_codes is keyed by `code` (text PRIMARY KEY), the
// canonical 12-character Crockford code.  All operations are single-row point
// reads/writes except RedeemedByUserID and AnonymiseRedemptionsByUserID, which
// use a partial index on redeemed_by_user_id.
//
// GDPR tombstone invariant: the `redeemed` boolean column is the authoritative
// consumed flag. AnonymiseRedemptionsByUserID scrubs redeemed_by_user_id and
// redeemed_at (PII) but always sets redeemed=true, so an erased code can never
// be re-redeemed. The RedeemWithCAS predicate is `redeemed = false` — NOT
// `redeemed_by_user_id IS NULL` — because an anonymised code has redeemed=true
// with a NULL redeemed_by_user_id.
type PostgresStore struct {
	db querier
}

// NewPostgresStore returns an offer-code store over the given pgx pool (or any
// querier that satisfies the interface).
func NewPostgresStore(db querier) *PostgresStore {
	return &PostgresStore{db: db}
}

// pgCodeColumns is the projection used by all single-row and multi-row reads.
// Its order MUST match the scan targets in scanCode.
const pgCodeColumns = "code, tier, duration_days, created_at, redeemed, redeemed_by_user_id, redeemed_at"

// scanCode hydrates one OfferCode from a single pgx.Row.
//
// Legacy-coalesce rule (mirroring offerCodeDocument.toDomain): a row with
// redeemed=false but a non-NULL redeemed_by_user_id was written before the
// redeemed boolean was added.  It must be treated as redeemed so the code
// cannot be re-claimed.
func scanCode(row pgx.Row) (OfferCode, error) {
	var (
		code             string
		tier             string
		durationDays     int
		createdAt        time.Time
		redeemed         bool
		redeemedByUserID *string
		redeemedAt       *time.Time
	)
	if err := row.Scan(&code, &tier, &durationDays, &createdAt, &redeemed,
		&redeemedByUserID, &redeemedAt); err != nil {
		return OfferCode{}, err
	}
	t, err := profiles.ParseSubscriptionTier(tier)
	if err != nil {
		return OfferCode{}, fmt.Errorf("parse tier %q: %w", tier, err)
	}
	return OfferCode{
		Code:             code,
		Tier:             t,
		DurationDays:     durationDays,
		CreatedAt:        createdAt,
		Redeemed:         redeemed || redeemedByUserID != nil, // legacy coalesce
		RedeemedByUserID: redeemedByUserID,
		RedeemedAt:       redeemedAt,
	}, nil
}

// scanCodeRow adapts scanCode for use inside pgx.CollectRows over a multi-row
// result (pgx.CollectableRow satisfies pgx.Row because it has Scan).
func scanCodeRow(row pgx.CollectableRow) (OfferCode, error) {
	return scanCode(row)
}

// ─── Get ────────────────────────────────────────────────────────────────────

const pgGetCodeQuery = "SELECT " + pgCodeColumns +
	" FROM offer_codes WHERE code = $1"

// Get point-reads the code; a miss surfaces as ErrNotFound.
func (s *PostgresStore) Get(ctx context.Context, canonical string) (OfferCode, error) {
	c, err := scanCode(s.db.QueryRow(ctx, pgGetCodeQuery, canonical))
	if errors.Is(err, pgx.ErrNoRows) {
		return OfferCode{}, ErrNotFound
	}
	if err != nil {
		return OfferCode{}, fmt.Errorf("read offer code %q: %w", canonical, err)
	}
	return c, nil
}

// ─── Save ───────────────────────────────────────────────────────────────────

const pgSaveCodeQuery = `
INSERT INTO offer_codes (code, tier, duration_days, created_at, redeemed, redeemed_by_user_id, redeemed_at)
VALUES ($1, $2, $3, $4, $5, $6, $7)
ON CONFLICT (code) DO UPDATE SET
    tier                = EXCLUDED.tier,
    duration_days       = EXCLUDED.duration_days,
    created_at          = EXCLUDED.created_at,
    redeemed            = EXCLUDED.redeemed,
    redeemed_by_user_id = EXCLUDED.redeemed_by_user_id,
    redeemed_at         = EXCLUDED.redeemed_at`

// Save upserts the offer code, keyed on code (PRIMARY KEY). Both fresh codes
// and post-redemption updates use this path.
func (s *PostgresStore) Save(ctx context.Context, c OfferCode) error {
	_, err := s.db.Exec(ctx, pgSaveCodeQuery,
		c.Code, c.Tier.String(), c.DurationDays, c.CreatedAt,
		c.IsRedeemed(), c.RedeemedByUserID, c.RedeemedAt,
	)
	if err != nil {
		return fmt.Errorf("upsert offer code %q: %w", c.Code, err)
	}
	return nil
}

// ─── RedeemWithCAS ──────────────────────────────────────────────────────────

// pgRedeemCASQuery is the atomic single-statement redeem. The WHERE predicate
// `redeemed = false` is the CAS guard: it matches only when the code exists
// AND has not yet been redeemed (including anonymised codes, which have
// redeemed=true). RETURNING code lets the caller detect "no row matched" via
// pgx.ErrNoRows without a separate SELECT.
const pgRedeemCASQuery = `
UPDATE offer_codes
SET redeemed = true, redeemed_by_user_id = $2, redeemed_at = $3
WHERE code = $1 AND redeemed = false
RETURNING code`

// pgCodeExistsQuery is the follow-up read when the CAS UPDATE matched no rows,
// distinguishing "code absent" (ErrNotFound) from "code already redeemed"
// (ErrAlreadyRedeemed). Scanning just `redeemed` avoids the full projection.
const pgCodeExistsQuery = "SELECT redeemed FROM offer_codes WHERE code = $1"

// RedeemWithCAS atomically redeems the offer code using a single
// UPDATE WHERE code=$1 AND redeemed=false RETURNING code statement.
//
// Returns:
//   - nil on success (row updated).
//   - ErrNotFound if the code does not exist.
//   - ErrAlreadyRedeemed if the code is already consumed (including codes that
//     were anonymised after GDPR erasure, which keep redeemed=true).
func (s *PostgresStore) RedeemWithCAS(ctx context.Context, canonical, userID string, now time.Time) error {
	var returnedCode string
	err := s.db.QueryRow(ctx, pgRedeemCASQuery, canonical, userID, now).Scan(&returnedCode)
	if err == nil {
		// UPDATE matched and modified exactly one row — success.
		return nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return fmt.Errorf("redeem offer code %q: %w", canonical, err)
	}

	// UPDATE matched no rows: either the code does not exist or it is already
	// redeemed. Run a lightweight existence check to decide.
	var redeemed bool
	checkErr := s.db.QueryRow(ctx, pgCodeExistsQuery, canonical).Scan(&redeemed)
	if errors.Is(checkErr, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if checkErr != nil {
		return fmt.Errorf("check offer code %q existence: %w", canonical, checkErr)
	}
	return ErrAlreadyRedeemed
}

// ─── RedeemedByUserID ────────────────────────────────────────────────────────

const pgRedeemedByUserIDQuery = "SELECT " + pgCodeColumns +
	" FROM offer_codes WHERE redeemed_by_user_id = $1"

// RedeemedByUserID returns every code the user has redeemed, hydrated to the
// domain model, for the GDPR data export (GET /v1/me/data). The common case —
// the user never redeemed a code — returns an empty, non-nil slice.
func (s *PostgresStore) RedeemedByUserID(ctx context.Context, userID string) ([]OfferCode, error) {
	rows, err := s.db.Query(ctx, pgRedeemedByUserIDQuery, userID)
	if err != nil {
		return nil, fmt.Errorf("query redemptions for user %q: %w", userID, err)
	}
	codes, err := pgx.CollectRows(rows, scanCodeRow)
	if err != nil {
		return nil, fmt.Errorf("scan redemptions for user %q: %w", userID, err)
	}
	if codes == nil {
		codes = []OfferCode{}
	}
	return codes, nil
}

// ─── AnonymiseRedemptionsByUserID ────────────────────────────────────────────

// pgAnonymiseByUserIDQuery scrubs the redeemer PII from every code the user
// redeemed.  CRITICAL: redeemed is set to true explicitly so the tombstone
// survives for codes where the boolean was absent or false in legacy data.
// This means an erased code can never be re-redeemed even though its
// redeemed_by_user_id is now NULL.
const pgAnonymiseByUserIDQuery = `
UPDATE offer_codes
SET redeemed = true, redeemed_by_user_id = NULL, redeemed_at = NULL
WHERE redeemed_by_user_id = $1`

// AnonymiseRedemptionsByUserID scrubs the redeemer back-reference
// (redeemed_by_user_id + redeemed_at) from every code the user redeemed, for
// UK GDPR Art. 17 account erasure. The consumed tombstone (redeemed=true) is
// preserved so the code can never be re-redeemed after erasure. The common
// case (the user never redeemed a code) matches zero rows and is a no-op.
func (s *PostgresStore) AnonymiseRedemptionsByUserID(ctx context.Context, userID string) error {
	_, err := s.db.Exec(ctx, pgAnonymiseByUserIDQuery, userID)
	if err != nil {
		return fmt.Errorf("anonymise redemptions for user %q: %w", userID, err)
	}
	return nil
}
