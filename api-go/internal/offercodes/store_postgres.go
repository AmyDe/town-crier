package offercodes

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/AmyDe/town-crier/api-go/internal/profiles"
)

// pgUniqueViolationCode is the Postgres SQLSTATE for a unique-constraint
// violation. RedeemWithCAS uses it to detect that the caller has already
// redeemed this code (the partial unique index on
// offer_code_redemptions(code, user_id)).
const pgUniqueViolationCode = "23505"

// pgForeignKeyViolationCode is the Postgres SQLSTATE for a foreign-key
// violation. RedeemWithCAS uses it to detect that the code does not exist at
// all: the offer_code_redemptions.code column references offer_codes(code),
// so inserting a redemption for an unknown code fails here rather than at the
// later UPDATE step.
const pgForeignKeyViolationCode = "23503"

// querier is the consumer-side slice of *pgxpool.Pool the Postgres offer-code
// store uses for single-statement operations. Both *pgxpool.Pool and pgx.Tx
// satisfy it structurally, so the store is decoupled from the concrete pool
// type.
type querier interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// pool is the full consumer-side interface the Postgres offer-code store
// needs: point reads/writes via querier, plus transactions. RedeemWithCAS and
// AnonymiseRedemptionsByUserID each need a real transaction — the insert into
// offer_code_redemptions and the paired offer_codes UPDATE must commit or roll
// back together, so a crash (or a rejected UPDATE) between them can never
// leave an orphan child row with no matching counter change. *pgxpool.Pool
// satisfies this interface structurally.
type pool interface {
	querier
	Begin(ctx context.Context) (pgx.Tx, error)
}

// Store is the full offer-code method set *PostgresStore satisfies and the
// exported consumer-side interface cmd/api's wiring accepts.
//
// The set covers:
//   - the codeStore interface in handler.go (Get/Save/RedeemWithCAS)
//   - the admin offerCodeStore interface in admin/handler.go (Save/List)
//   - erasure.RedemptionAnonymiser (AnonymiseRedemptionsByUserID)
//   - the GDPR export reader (RedeemedByUserID)
type Store interface {
	Get(ctx context.Context, canonical string) (OfferCode, error)
	Save(ctx context.Context, c OfferCode) error
	RedeemWithCAS(ctx context.Context, canonical, userID string, now time.Time) error
	RedeemedByUserID(ctx context.Context, userID string) ([]RedeemedOfferCode, error)
	AnonymiseRedemptionsByUserID(ctx context.Context, userID string) error
}

// Compile-time check: the store satisfies the exported Store interface.
var _ Store = (*PostgresStore)(nil)

// PostgresStore reads and writes offer codes in Postgres: the parent
// offer_codes table (one row per code, keyed by the canonical 12-character
// Crockford code) and the child offer_code_redemptions table (one row per
// user who has redeemed a code).
//
// Multi-redemption model (GH#866): every code carries a MaxRedemptions cap
// (1 for the single-use case) and a RedemptionCount tombstone counter that is
// never decremented. A redemption inserts a row into offer_code_redemptions —
// enforcing "one redemption per user per code" via a partial unique index on
// (code, user_id) — and atomically increments the counter, guarded by
// `redemption_count < max_redemptions`. GDPR erasure
// (AnonymiseRedemptionsByUserID) nulls a redemption row's user_id/redeemed_at
// but never decrements the counter, so a consumed slot can never be reclaimed.
//
// Legacy columns (redeemed, redeemed_by_user_id, redeemed_at) stay on
// offer_codes and are dual-written by RedeemWithCAS and scrubbed by
// AnonymiseRedemptionsByUserID: a staged-rollout safety net so a rolled-back
// binary still recognises a consumed code (see migration 0019). They are
// never read back into the domain model — RedemptionCount/MaxRedemptions are
// authoritative going forward.
type PostgresStore struct {
	db     pool
	logger *slog.Logger
}

// NewPostgresStore returns an offer-code store over the given pgx pool (or any
// pool that satisfies the interface). logger receives a record whenever a
// deferred transaction rollback in RedeemWithCAS or
// AnonymiseRedemptionsByUserID fails for a reason other than the transaction
// already having committed (see the doc comments on those methods).
func NewPostgresStore(db pool, logger *slog.Logger) *PostgresStore {
	return &PostgresStore{db: db, logger: logger}
}

// pgCodeColumns is the projection used by Get. Its order MUST match the scan
// targets in scanCode.
const pgCodeColumns = "code, tier, duration_days, created_at, label, max_redemptions, redemption_count"

// scanCode hydrates one OfferCode from a single pgx.Row. label is nullable in
// the schema (rows minted before this column existed, or never given one) and
// coalesces to the empty string.
func scanCode(row pgx.Row) (OfferCode, error) {
	var (
		code            string
		tier            string
		durationDays    int
		createdAt       time.Time
		label           *string
		maxRedemptions  int
		redemptionCount int
	)
	if err := row.Scan(&code, &tier, &durationDays, &createdAt, &label,
		&maxRedemptions, &redemptionCount); err != nil {
		return OfferCode{}, err
	}
	t, err := profiles.ParseSubscriptionTier(tier)
	if err != nil {
		return OfferCode{}, fmt.Errorf("parse tier %q: %w", tier, err)
	}
	lbl := ""
	if label != nil {
		lbl = *label
	}
	return OfferCode{
		Code:            code,
		Tier:            t,
		DurationDays:    durationDays,
		CreatedAt:       createdAt,
		Label:           lbl,
		MaxRedemptions:  maxRedemptions,
		RedemptionCount: redemptionCount,
	}, nil
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
INSERT INTO offer_codes (code, tier, duration_days, created_at, label, max_redemptions)
VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT (code) DO UPDATE SET
    tier            = EXCLUDED.tier,
    duration_days   = EXCLUDED.duration_days,
    created_at      = EXCLUDED.created_at,
    label           = EXCLUDED.label,
    max_redemptions = EXCLUDED.max_redemptions`

// Save upserts the code's mint/update metadata (code, tier, duration, created
// at, label, max redemptions) only — it never writes redemption state
// (redemption_count and the legacy redeemed* columns are left untouched on a
// conflict, so re-saving a code's metadata can never reset or fabricate its
// redemption history).
func (s *PostgresStore) Save(ctx context.Context, c OfferCode) error {
	_, err := s.db.Exec(ctx, pgSaveCodeQuery,
		c.Code, c.Tier.String(), c.DurationDays, c.CreatedAt, c.Label, c.MaxRedemptions,
	)
	if err != nil {
		return fmt.Errorf("upsert offer code %q: %w", c.Code, err)
	}
	return nil
}

// ─── RedeemWithCAS ──────────────────────────────────────────────────────────

// pgInsertRedemptionQuery inserts the child redemption row. A unique
// violation on (code, user_id) — the partial unique index — means userID has
// already redeemed this code.
const pgInsertRedemptionQuery = `
INSERT INTO offer_code_redemptions (code, user_id, redeemed_at)
VALUES ($1, $2, $3)`

// pgRedeemUpdateQuery is the atomic redemption-cap guard: it matches only when
// the code exists and has at least one free slot
// (redemption_count < max_redemptions). RETURNING code lets the caller detect
// "no row matched" via pgx.ErrNoRows without a separate SELECT. The COALESCE
// calls dual-write the legacy redeemed/redeemed_by_user_id/redeemed_at
// columns (first-redeemer semantics preserved) so a rolled-back binary still
// recognises the code as consumed — see migration 0019.
const pgRedeemUpdateQuery = `
UPDATE offer_codes
SET redemption_count    = redemption_count + 1,
    redeemed            = true,
    redeemed_by_user_id = COALESCE(redeemed_by_user_id, $2),
    redeemed_at         = COALESCE(redeemed_at, $3)
WHERE code = $1 AND redemption_count < max_redemptions
RETURNING code`

// pgCodeExistsQuery is the follow-up read when the UPDATE matched no rows,
// distinguishing "code absent" (ErrNotFound) from "code fully consumed"
// (ErrAlreadyRedeemed).
const pgCodeExistsQuery = "SELECT 1 FROM offer_codes WHERE code = $1"

// RedeemWithCAS atomically redeems the offer code for userID at now, as a
// single transaction:
//  1. INSERT into offer_code_redemptions(code, user_id, redeemed_at). A
//     foreign-key violation (the code does not exist) returns ErrNotFound; a
//     unique violation (userID already redeemed this code) returns
//     ErrAlreadyRedeemedByUser. Neither touches offer_codes.
//  2. UPDATE offer_codes ... WHERE redemption_count < max_redemptions
//     RETURNING code. No matched row means every slot is already consumed
//     (ErrAlreadyRedeemed) — the existing existence-check pattern used
//     elsewhere in this file confirms the code is present (it must be, having
//     just passed the FK check in step 1) before returning it. Either way the
//     transaction is rolled back, undoing step 1's insert so a redemption
//     that didn't actually grant anything is never recorded.
//
// On success both writes commit together.
func (s *PostgresStore) RedeemWithCAS(ctx context.Context, canonical, userID string, now time.Time) error {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin redeem transaction for %q: %w", canonical, err)
	}
	defer s.rollback(ctx, tx) // no-op once committed

	if _, err := tx.Exec(ctx, pgInsertRedemptionQuery, canonical, userID, now); err != nil {
		switch {
		case isUniqueViolation(err):
			return ErrAlreadyRedeemedByUser
		case isForeignKeyViolation(err):
			return ErrNotFound
		default:
			return fmt.Errorf("insert redemption for %q: %w", canonical, err)
		}
	}

	var returnedCode string
	err = tx.QueryRow(ctx, pgRedeemUpdateQuery, canonical, userID, now).Scan(&returnedCode)
	if err == nil {
		if commitErr := tx.Commit(ctx); commitErr != nil {
			return fmt.Errorf("commit redemption for %q: %w", canonical, commitErr)
		}
		return nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return fmt.Errorf("redeem offer code %q: %w", canonical, err)
	}

	// UPDATE matched no rows: either the code does not exist or every slot is
	// already consumed. Run a lightweight existence check to decide.
	var exists int
	checkErr := tx.QueryRow(ctx, pgCodeExistsQuery, canonical).Scan(&exists)
	if errors.Is(checkErr, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if checkErr != nil {
		return fmt.Errorf("check offer code %q existence: %w", canonical, checkErr)
	}
	return ErrAlreadyRedeemed
}

// isUniqueViolation reports whether err is a Postgres unique-constraint
// violation (SQLSTATE 23505).
func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == pgUniqueViolationCode
	}
	return false
}

// isForeignKeyViolation reports whether err is a Postgres foreign-key
// violation (SQLSTATE 23503).
func isForeignKeyViolation(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == pgForeignKeyViolationCode
	}
	return false
}

// rollback runs tx's deferred, best-effort Rollback. On the success path (the
// caller already committed) pgx's documented behaviour is that Rollback
// becomes a no-op returning pgx.ErrTxClosed — that case is expected and not
// logged. Any other error means the rollback did not actually happen, so the
// transaction's locks/resources may still be held; that is logged (not
// returned) because this always runs from a defer, after the method has
// already produced its real result.
func (s *PostgresStore) rollback(ctx context.Context, tx pgx.Tx) {
	if err := tx.Rollback(ctx); err != nil && !errors.Is(err, pgx.ErrTxClosed) {
		s.logger.ErrorContext(ctx, "offer code store: transaction rollback failed", "error", err)
	}
}

// ─── RedeemedByUserID / RedeemedByUsers ─────────────────────────────────────

// pgRedeemedColumns is the join projection shared by RedeemedByUserID and
// RedeemedByUsers. Its order MUST match the scan targets in
// scanRedeemedOfferCode.
const pgRedeemedColumns = "c.code, c.tier, c.duration_days, r.redeemed_at, r.user_id"

// scanRedeemedOfferCode hydrates one RedeemedOfferCode from a joined
// offer_code_redemptions/offer_codes row.
func scanRedeemedOfferCode(row pgx.Row) (RedeemedOfferCode, error) {
	var (
		code         string
		tier         string
		durationDays int
		redeemedAt   *time.Time
		userID       *string
	)
	if err := row.Scan(&code, &tier, &durationDays, &redeemedAt, &userID); err != nil {
		return RedeemedOfferCode{}, err
	}
	t, err := profiles.ParseSubscriptionTier(tier)
	if err != nil {
		return RedeemedOfferCode{}, fmt.Errorf("parse tier %q: %w", tier, err)
	}
	return RedeemedOfferCode{
		Code: code, Tier: t, DurationDays: durationDays, RedeemedAt: redeemedAt, UserID: userID,
	}, nil
}

func scanRedeemedOfferCodeRow(row pgx.CollectableRow) (RedeemedOfferCode, error) {
	return scanRedeemedOfferCode(row)
}

const pgRedeemedByUserIDQuery = `
SELECT ` + pgRedeemedColumns + `
FROM offer_code_redemptions r JOIN offer_codes c ON c.code = r.code
WHERE r.user_id = $1
ORDER BY r.id`

// RedeemedByUserID returns every redemption the user has made, joined with
// each code's tier/duration, for the GDPR data export (GET /v1/me/data). The
// common case — the user never redeemed a code — returns an empty, non-nil
// slice.
func (s *PostgresStore) RedeemedByUserID(ctx context.Context, userID string) ([]RedeemedOfferCode, error) {
	rows, err := s.db.Query(ctx, pgRedeemedByUserIDQuery, userID)
	if err != nil {
		return nil, fmt.Errorf("query redemptions for user %q: %w", userID, err)
	}
	codes, err := pgx.CollectRows(rows, scanRedeemedOfferCodeRow)
	if err != nil {
		return nil, fmt.Errorf("scan redemptions for user %q: %w", userID, err)
	}
	if codes == nil {
		codes = []RedeemedOfferCode{}
	}
	return codes, nil
}

const pgRedeemedByUsersQuery = `
SELECT ` + pgRedeemedColumns + `
FROM offer_code_redemptions r JOIN offer_codes c ON c.code = r.code
WHERE r.user_id = ANY($1)
ORDER BY r.id`

// RedeemedByUsers returns, for each user id in the set, the redemptions that
// user has made — the batched form of RedeemedByUserID used by the admin user
// list. It issues one grouped query instead of N per-user round trips, then
// buckets the flat result in Go by each row's UserID.
//
// Users who never redeemed a code are absent from the map (the caller treats a
// missing key as "no active offer" via the map zero value). An anonymised
// redemption has a NULL user_id, so it never matches ANY($1) and never leaks a
// GDPR-erased user back into the result. An empty user set returns an empty,
// non-nil map without issuing a query.
func (s *PostgresStore) RedeemedByUsers(ctx context.Context, userIDs []string) (map[string][]RedeemedOfferCode, error) {
	byUser := make(map[string][]RedeemedOfferCode, len(userIDs))
	if len(userIDs) == 0 {
		return byUser, nil
	}
	rows, err := s.db.Query(ctx, pgRedeemedByUsersQuery, userIDs)
	if err != nil {
		return nil, fmt.Errorf("query redemptions for users: %w", err)
	}
	codes, err := pgx.CollectRows(rows, scanRedeemedOfferCodeRow)
	if err != nil {
		return nil, fmt.Errorf("scan redemptions for users: %w", err)
	}
	for _, c := range codes {
		if c.UserID == nil {
			continue // defensive: the ANY predicate already excludes NULL redeemers
		}
		byUser[*c.UserID] = append(byUser[*c.UserID], c)
	}
	return byUser, nil
}

// ─── AnonymiseRedemptionsByUserID ────────────────────────────────────────────

// pgAnonymiseChildQuery scrubs the redeemer PII from every redemption row the
// user made, keeping the row itself as a tombstone (redemption_count is never
// touched, so the slot stays consumed).
const pgAnonymiseChildQuery = `
UPDATE offer_code_redemptions
SET user_id = NULL, redeemed_at = NULL
WHERE user_id = $1`

// pgAnonymiseLegacyQuery scrubs the dual-written legacy columns on
// offer_codes. CRITICAL: redeemed is set to true explicitly so the tombstone
// survives for codes where the boolean was absent or false in legacy data.
const pgAnonymiseLegacyQuery = `
UPDATE offer_codes
SET redeemed = true, redeemed_by_user_id = NULL, redeemed_at = NULL
WHERE redeemed_by_user_id = $1`

// AnonymiseRedemptionsByUserID scrubs the redeemer PII (user_id + redeemed_at)
// from every redemption the user made, for UK GDPR Art. 17 account erasure —
// both on the authoritative child table and the dual-written legacy columns on
// offer_codes. RedemptionCount is never decremented, so a consumed slot can
// never be reclaimed after erasure. The common case (the user never redeemed a
// code) matches zero rows in both statements and is a no-op. Both writes run
// in one transaction so a failure never leaves the child table scrubbed while
// the legacy dual-write is stale, or vice versa.
func (s *PostgresStore) AnonymiseRedemptionsByUserID(ctx context.Context, userID string) error {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin anonymise transaction for user %q: %w", userID, err)
	}
	defer s.rollback(ctx, tx) // no-op once committed

	if _, err := tx.Exec(ctx, pgAnonymiseChildQuery, userID); err != nil {
		return fmt.Errorf("anonymise redemption rows for user %q: %w", userID, err)
	}
	if _, err := tx.Exec(ctx, pgAnonymiseLegacyQuery, userID); err != nil {
		return fmt.Errorf("anonymise legacy offer-code columns for user %q: %w", userID, err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit anonymise for user %q: %w", userID, err)
	}
	return nil
}

// ─── List ───────────────────────────────────────────────────────────────────

// pgListCodesQuery lists codes newest-first, optionally filtered by a
// case-insensitive label substring. $1::text IS NULL short-circuits the
// filter when labelFilter is nil, so a single static query serves both the
// filtered and unfiltered cases.
const pgListCodesQuery = `
SELECT c.code, c.tier, c.duration_days, c.created_at, c.label, c.max_redemptions, c.redemption_count,
       (SELECT max(r.redeemed_at) FROM offer_code_redemptions r WHERE r.code = c.code) AS last_redeemed_at
FROM offer_codes c
WHERE $1::text IS NULL OR c.label ILIKE '%' || $1 || '%'
ORDER BY c.created_at DESC
LIMIT $2`

// List returns codes ordered created_at DESC, each annotated with its most
// recent redemption time (nil if never redeemed), optionally filtered to
// labels containing labelFilter (case-insensitive substring match). limit
// bounds the result set; there is no pagination beyond it (the table is
// admin-minted and small).
func (s *PostgresStore) List(ctx context.Context, labelFilter *string, limit int) ([]ListedOfferCode, error) {
	rows, err := s.db.Query(ctx, pgListCodesQuery, labelFilter, limit)
	if err != nil {
		return nil, fmt.Errorf("list offer codes: %w", err)
	}
	listed, err := pgx.CollectRows(rows, scanListedOfferCodeRow)
	if err != nil {
		return nil, fmt.Errorf("scan listed offer codes: %w", err)
	}
	if listed == nil {
		listed = []ListedOfferCode{}
	}
	return listed, nil
}

func scanListedOfferCodeRow(row pgx.CollectableRow) (ListedOfferCode, error) {
	var (
		code            string
		tier            string
		durationDays    int
		createdAt       time.Time
		label           *string
		maxRedemptions  int
		redemptionCount int
		lastRedeemedAt  *time.Time
	)
	if err := row.Scan(&code, &tier, &durationDays, &createdAt, &label,
		&maxRedemptions, &redemptionCount, &lastRedeemedAt); err != nil {
		return ListedOfferCode{}, err
	}
	t, err := profiles.ParseSubscriptionTier(tier)
	if err != nil {
		return ListedOfferCode{}, fmt.Errorf("parse tier %q: %w", tier, err)
	}
	lbl := ""
	if label != nil {
		lbl = *label
	}
	return ListedOfferCode{
		OfferCode: OfferCode{
			Code: code, Tier: t, DurationDays: durationDays, CreatedAt: createdAt,
			Label: lbl, MaxRedemptions: maxRedemptions, RedemptionCount: redemptionCount,
		},
		LastRedeemedAt: lastRedeemedAt,
	}, nil
}
