//go:build integration

package offercodes

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/AmyDe/town-crier/api-go/internal/platform/postgres/pgtest"
	"github.com/AmyDe/town-crier/api-go/internal/profiles"
)

// newPGStore returns a PostgresStore over a truncated, migrated test database.
// Integration tests MUST NOT call t.Parallel: the pgtest harness holds a
// session-level advisory lock so all integration tests in all packages serialise
// on the single docker-compose database (see pgtest.New doc).
func newPGStore(t *testing.T) *PostgresStore {
	t.Helper()
	pool := pgtest.New(t)
	pgtest.Truncate(t, pool, "offer_code_redemptions", "offer_codes")
	return NewPostgresStore(pool)
}

// testCode returns a valid, unredeemed single-use OfferCode for use in
// integration tests.
func testCode(t *testing.T, code string) OfferCode {
	t.Helper()
	return testCodeWithCap(t, code, 1)
}

// testCodeWithCap returns a valid, unredeemed OfferCode minted with the given
// redemption cap.
func testCodeWithCap(t *testing.T, code string, maxRedemptions int) OfferCode {
	t.Helper()
	c, err := NewOfferCode(code, profiles.TierPro, 30, "integration-test", maxRedemptions,
		time.Date(2026, 6, 26, 12, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("NewOfferCode(%q): %v", code, err)
	}
	return c
}

// TestPostgresStore_RoundTrip writes an unredeemed code and reads it back,
// asserting that all fields survive the Postgres round-trip intact.
func TestPostgresStore_RoundTrip(t *testing.T) {
	ctx := context.Background()
	store := newPGStore(t)

	code := testCode(t, "ABCDEFGHJKMN")
	if err := store.Save(ctx, code); err != nil {
		t.Fatalf("Save: %v", err)
	}

	got, err := store.Get(ctx, "ABCDEFGHJKMN")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Code != "ABCDEFGHJKMN" {
		t.Errorf("Code: got %q, want ABCDEFGHJKMN", got.Code)
	}
	if got.Tier != profiles.TierPro {
		t.Errorf("Tier: got %v, want TierPro", got.Tier)
	}
	if got.DurationDays != 30 {
		t.Errorf("DurationDays: got %d, want 30", got.DurationDays)
	}
	if got.Label != "integration-test" {
		t.Errorf("Label: got %q, want integration-test", got.Label)
	}
	if got.MaxRedemptions != 1 || got.RedemptionCount != 0 {
		t.Errorf("MaxRedemptions/RedemptionCount: got %d/%d, want 1/0", got.MaxRedemptions, got.RedemptionCount)
	}
	if got.IsFullyRedeemed() {
		t.Error("freshly stored code must not be fully redeemed")
	}
}

// TestPostgresStore_Get_Miss_Integration confirms that reading a code that was
// never saved returns ErrNotFound.
func TestPostgresStore_Get_Miss_Integration(t *testing.T) {
	ctx := context.Background()
	store := newPGStore(t)

	_, err := store.Get(ctx, "ZZZZZZZZZZZZ")
	if err == nil || !isErr(err, ErrNotFound) {
		t.Fatalf("Get miss: got %v, want ErrNotFound", err)
	}
}

// TestPostgresStore_RedeemRoundTrip saves a code, redeems it via RedeemWithCAS,
// reads it back, and asserts that the redemption fields (RedemptionCount, plus
// the dual-written legacy columns) are persisted.
func TestPostgresStore_RedeemRoundTrip(t *testing.T) {
	ctx := context.Background()
	store := newPGStore(t)

	code := testCode(t, "ABCDEFGHJKMN")
	if err := store.Save(ctx, code); err != nil {
		t.Fatalf("Save: %v", err)
	}

	now := time.Date(2026, 6, 26, 13, 0, 0, 0, time.UTC)
	if err := store.RedeemWithCAS(ctx, "ABCDEFGHJKMN", "auth0|u1", now); err != nil {
		t.Fatalf("RedeemWithCAS: %v", err)
	}

	got, err := store.Get(ctx, "ABCDEFGHJKMN")
	if err != nil {
		t.Fatalf("Get after redeem: %v", err)
	}
	if !got.IsFullyRedeemed() {
		t.Error("single-use code must be fully redeemed after RedeemWithCAS")
	}
	if got.RedemptionCount != 1 {
		t.Errorf("RedemptionCount: got %d, want 1", got.RedemptionCount)
	}

	redemptions, err := store.RedeemedByUserID(ctx, "auth0|u1")
	if err != nil {
		t.Fatalf("RedeemedByUserID: %v", err)
	}
	if len(redemptions) != 1 || redemptions[0].Code != "ABCDEFGHJKMN" {
		t.Fatalf("redemptions: got %+v, want one row for ABCDEFGHJKMN", redemptions)
	}
	if redemptions[0].RedeemedAt == nil || !redemptions[0].RedeemedAt.UTC().Equal(now.UTC()) {
		t.Errorf("RedeemedAt: got %v, want %v", redemptions[0].RedeemedAt, now)
	}
}

// TestPostgresStore_RedeemWithCAS_AlreadyRedeemed_Integration confirms that a
// second distinct user redeeming an already-fully-consumed single-use code
// returns ErrAlreadyRedeemed.
func TestPostgresStore_RedeemWithCAS_AlreadyRedeemed_Integration(t *testing.T) {
	ctx := context.Background()
	store := newPGStore(t)

	code := testCode(t, "ABCDEFGHJKMN")
	if err := store.Save(ctx, code); err != nil {
		t.Fatalf("Save: %v", err)
	}
	now := time.Now().UTC()
	if err := store.RedeemWithCAS(ctx, "ABCDEFGHJKMN", "auth0|u1", now); err != nil {
		t.Fatalf("first RedeemWithCAS: %v", err)
	}
	if err := store.RedeemWithCAS(ctx, "ABCDEFGHJKMN", "auth0|u2", now); !isErr(err, ErrAlreadyRedeemed) {
		t.Fatalf("second RedeemWithCAS: got %v, want ErrAlreadyRedeemed", err)
	}
}

// TestPostgresStore_RedeemWithCAS_AlreadyRedeemedByUser_Integration confirms
// that the SAME user redeeming the same code twice returns
// ErrAlreadyRedeemedByUser — distinct from the fully-consumed case above, even
// though a multi-use code still has free slots for other users.
func TestPostgresStore_RedeemWithCAS_AlreadyRedeemedByUser_Integration(t *testing.T) {
	ctx := context.Background()
	store := newPGStore(t)

	code := testCodeWithCap(t, "ABCDEFGHJKMN", 3)
	if err := store.Save(ctx, code); err != nil {
		t.Fatalf("Save: %v", err)
	}
	now := time.Now().UTC()
	if err := store.RedeemWithCAS(ctx, "ABCDEFGHJKMN", "auth0|u1", now); err != nil {
		t.Fatalf("first RedeemWithCAS: %v", err)
	}
	if err := store.RedeemWithCAS(ctx, "ABCDEFGHJKMN", "auth0|u1", now); !isErr(err, ErrAlreadyRedeemedByUser) {
		t.Fatalf("same-user second RedeemWithCAS: got %v, want ErrAlreadyRedeemedByUser", err)
	}

	// The wasted insert attempt must have rolled back: the code's redemption
	// count stays at 1, not 2.
	got, err := store.Get(ctx, "ABCDEFGHJKMN")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.RedemptionCount != 1 {
		t.Errorf("RedemptionCount after rejected same-user redeem: got %d, want 1", got.RedemptionCount)
	}
}

// TestPostgresStore_RedeemWithCAS_NotFound_Integration confirms that calling
// RedeemWithCAS on a non-existent code returns ErrNotFound.
func TestPostgresStore_RedeemWithCAS_NotFound_Integration(t *testing.T) {
	ctx := context.Background()
	store := newPGStore(t)

	if err := store.RedeemWithCAS(ctx, "ZZZZZZZZZZZZ", "auth0|u1", time.Now().UTC()); !isErr(err, ErrNotFound) {
		t.Fatalf("RedeemWithCAS not found: got %v, want ErrNotFound", err)
	}
}

// TestPostgresStore_MultiRedemption_AcceptsUpToCapThenRejects is the core
// multi-redemption contract (GH#866): a code minted with maxRedemptions=3
// accepts exactly three distinct redeemers, then rejects a fourth with
// ErrAlreadyRedeemed.
func TestPostgresStore_MultiRedemption_AcceptsUpToCapThenRejects(t *testing.T) {
	ctx := context.Background()
	store := newPGStore(t)

	code := testCodeWithCap(t, "ABCDEFGHJKMN", 3)
	if err := store.Save(ctx, code); err != nil {
		t.Fatalf("Save: %v", err)
	}
	now := time.Now().UTC()

	for _, userID := range []string{"auth0|u1", "auth0|u2", "auth0|u3"} {
		if err := store.RedeemWithCAS(ctx, "ABCDEFGHJKMN", userID, now); err != nil {
			t.Fatalf("RedeemWithCAS(%s): %v", userID, err)
		}
	}

	got, err := store.Get(ctx, "ABCDEFGHJKMN")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.RedemptionCount != 3 || !got.IsFullyRedeemed() {
		t.Fatalf("after 3 redemptions: got RedemptionCount=%d IsFullyRedeemed=%v, want 3/true",
			got.RedemptionCount, got.IsFullyRedeemed())
	}

	if err := store.RedeemWithCAS(ctx, "ABCDEFGHJKMN", "auth0|u4", now); !isErr(err, ErrAlreadyRedeemed) {
		t.Fatalf("4th redeemer: got %v, want ErrAlreadyRedeemed", err)
	}
	// The rejected 4th attempt must not have bumped the counter.
	got, err = store.Get(ctx, "ABCDEFGHJKMN")
	if err != nil {
		t.Fatalf("Get after rejected 4th: %v", err)
	}
	if got.RedemptionCount != 3 {
		t.Errorf("RedemptionCount after rejected 4th redeemer: got %d, want 3", got.RedemptionCount)
	}
}

// TestPostgresStore_RedeemRace_NeverExceedsCap is the required concurrency
// proof: many goroutines race to redeem a maxRedemptions=N code with more than
// N distinct users; the transaction (insert + capped UPDATE) must ensure
// exactly N succeed and the rest get ErrAlreadyRedeemed, never more than N.
func TestPostgresStore_RedeemRace_NeverExceedsCap(t *testing.T) {
	ctx := context.Background()
	store := newPGStore(t)

	const cap_ = 3
	const racers = 8
	code := testCodeWithCap(t, "ABCDEFGHJKMN", cap_)
	if err := store.Save(ctx, code); err != nil {
		t.Fatalf("Save: %v", err)
	}

	now := time.Now().UTC()
	errs := make([]error, racers)
	var wg sync.WaitGroup
	wg.Add(racers)
	for i := range racers {
		go func(i int) {
			defer wg.Done()
			userID := "auth0|racer" + string(rune('A'+i))
			errs[i] = store.RedeemWithCAS(ctx, "ABCDEFGHJKMN", userID, now)
		}(i)
	}
	wg.Wait()

	successes := 0
	alreadyRedeemed := 0
	for _, err := range errs {
		switch {
		case err == nil:
			successes++
		case isErr(err, ErrAlreadyRedeemed):
			alreadyRedeemed++
		default:
			t.Errorf("unexpected error in race: %v", err)
		}
	}
	if successes != cap_ {
		t.Errorf("race: got %d successes, want exactly %d", successes, cap_)
	}
	if alreadyRedeemed != racers-cap_ {
		t.Errorf("race: got %d ErrAlreadyRedeemed, want exactly %d", alreadyRedeemed, racers-cap_)
	}

	got, err := store.Get(ctx, "ABCDEFGHJKMN")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.RedemptionCount != cap_ {
		t.Fatalf("RedemptionCount after race: got %d, want exactly %d (never exceeds cap)", got.RedemptionCount, cap_)
	}
}

// TestPostgresStore_AnonymiseScrubsPIIKeepsTombstone verifies the full GDPR
// anonymise contract:
//  1. After AnonymiseRedemptionsByUserID the redemption row's user_id and
//     redeemed_at are NULL, and the dual-written legacy columns are scrubbed
//     the same way.
//  2. RedemptionCount is unchanged — the slot stays consumed, so a
//     subsequent RedeemWithCAS by a new user still respects the cap.
func TestPostgresStore_AnonymiseScrubsPIIKeepsTombstone(t *testing.T) {
	ctx := context.Background()
	store := newPGStore(t)

	// Save and redeem a single-use code.
	code := testCode(t, "ABCDEFGHJKMN")
	if err := store.Save(ctx, code); err != nil {
		t.Fatalf("Save: %v", err)
	}
	now := time.Now().UTC()
	if err := store.RedeemWithCAS(ctx, "ABCDEFGHJKMN", "auth0|target", now); err != nil {
		t.Fatalf("RedeemWithCAS: %v", err)
	}

	// Anonymise the redeemer.
	if err := store.AnonymiseRedemptionsByUserID(ctx, "auth0|target"); err != nil {
		t.Fatalf("AnonymiseRedemptionsByUserID: %v", err)
	}

	// The redemption count (tombstone) must survive unchanged.
	got, err := store.Get(ctx, "ABCDEFGHJKMN")
	if err != nil {
		t.Fatalf("Get after anonymise: %v", err)
	}
	if got.RedemptionCount != 1 {
		t.Errorf("RedemptionCount after anonymise: got %d, want 1 (never decremented)", got.RedemptionCount)
	}
	if !got.IsFullyRedeemed() {
		t.Error("IsFullyRedeemed() must stay true after anonymisation — the slot is still consumed")
	}

	// The redeemer's own export view must show nothing (PII scrubbed).
	redemptions, err := store.RedeemedByUserID(ctx, "auth0|target")
	if err != nil {
		t.Fatalf("RedeemedByUserID after anonymise: %v", err)
	}
	if len(redemptions) != 0 {
		t.Errorf("after anonymise: got %d redemptions for the target user, want 0", len(redemptions))
	}

	// A new user must still be rejected — the single slot stays consumed.
	if err := store.RedeemWithCAS(ctx, "ABCDEFGHJKMN", "auth0|new", time.Now().UTC()); !isErr(err, ErrAlreadyRedeemed) {
		t.Fatalf("post-anonymise RedeemWithCAS: got %v, want ErrAlreadyRedeemed", err)
	}
}

// TestPostgresStore_RedeemedByUserID_Integration confirms that RedeemedByUserID
// returns only the calling user's redemptions and returns an empty slice after
// AnonymiseRedemptionsByUserID removes the back-reference.
func TestPostgresStore_RedeemedByUserID_Integration(t *testing.T) {
	ctx := context.Background()
	store := newPGStore(t)

	// Save two codes and redeem them for different users.
	mine := testCode(t, "AAAAAAAAAAAA")
	theirs := testCode(t, "BBBBBBBBBBBB")
	for _, c := range []OfferCode{mine, theirs} {
		if err := store.Save(ctx, c); err != nil {
			t.Fatalf("Save %q: %v", c.Code, err)
		}
	}
	now := time.Now().UTC()
	if err := store.RedeemWithCAS(ctx, "AAAAAAAAAAAA", "auth0|target", now); err != nil {
		t.Fatalf("redeem mine: %v", err)
	}
	if err := store.RedeemWithCAS(ctx, "BBBBBBBBBBBB", "auth0|other", now); err != nil {
		t.Fatalf("redeem theirs: %v", err)
	}

	// RedeemedByUserID returns only the target's redemptions.
	got, err := store.RedeemedByUserID(ctx, "auth0|target")
	if err != nil {
		t.Fatalf("RedeemedByUserID: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("count: got %d, want 1", len(got))
	}
	if got[0].Code != "AAAAAAAAAAAA" {
		t.Errorf("code: got %q, want AAAAAAAAAAAA", got[0].Code)
	}

	// A user who never redeemed returns empty, non-nil.
	none, err := store.RedeemedByUserID(ctx, "auth0|never")
	if err != nil {
		t.Fatalf("RedeemedByUserID never: %v", err)
	}
	if none == nil || len(none) != 0 {
		t.Errorf("empty result: got %v, want non-nil empty slice", none)
	}
}

// TestPostgresStore_RedeemedByUsers_Integration seeds redemptions for two users
// (one with two codes) plus an unredeemed code and an anonymised (scrubbed)
// redemption, then asserts the batched ANY($1) query returns only the redeemed
// codes grouped by their redeemer — and never the anonymised NULL-redeemer row.
func TestPostgresStore_RedeemedByUsers_Integration(t *testing.T) {
	ctx := context.Background()
	store := newPGStore(t)

	now := time.Date(2026, 6, 26, 12, 0, 0, 0, time.UTC)
	// u1 redeems two codes; u2 redeems one.
	for code, user := range map[string]string{
		"AAAAAAAAAAAA": "auth0|u1",
		"CCCCCCCCCCCC": "auth0|u1",
		"BBBBBBBBBBBB": "auth0|u2",
	} {
		if err := store.Save(ctx, testCode(t, code)); err != nil {
			t.Fatalf("Save %s: %v", code, err)
		}
		if err := store.RedeemWithCAS(ctx, code, user, now); err != nil {
			t.Fatalf("Redeem %s: %v", code, err)
		}
	}
	// An unredeemed code must never appear.
	if err := store.Save(ctx, testCode(t, "DDDDDDDDDDDD")); err != nil {
		t.Fatalf("Save unredeemed: %v", err)
	}
	// An anonymised (GDPR-scrubbed) redemption keeps its row (NULL user_id), so
	// it must never appear in a by-user lookup.
	if err := store.Save(ctx, testCode(t, "EEEEEEEEEEEE")); err != nil {
		t.Fatalf("Save anon code: %v", err)
	}
	if err := store.RedeemWithCAS(ctx, "EEEEEEEEEEEE", "auth0|erased", now); err != nil {
		t.Fatalf("Redeem anon code: %v", err)
	}
	if err := store.AnonymiseRedemptionsByUserID(ctx, "auth0|erased"); err != nil {
		t.Fatalf("Anonymise: %v", err)
	}

	got, err := store.RedeemedByUsers(ctx, []string{"auth0|u1", "auth0|u2", "auth0|erased"})
	if err != nil {
		t.Fatalf("RedeemedByUsers: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("map size: got %d, want 2 (erased user absent)", len(got))
	}
	if len(got["auth0|u1"]) != 2 {
		t.Errorf("u1 codes: got %d, want 2", len(got["auth0|u1"]))
	}
	if len(got["auth0|u2"]) != 1 || got["auth0|u2"][0].Code != "BBBBBBBBBBBB" {
		t.Errorf("u2 codes: got %+v, want [BBBBBBBBBBBB]", got["auth0|u2"])
	}
	if _, ok := got["auth0|erased"]; ok {
		t.Error("anonymised redemption (NULL redeemer) must not appear")
	}
}

// TestPostgresStore_LegacyCoalesceBackfill_Integration mirrors migration
// 0019's backfill statements against three manually-seeded "pre-migration"
// rows — fresh, redeemed, and already-anonymised — each with
// redemption_count/child rows still at their post-ALTER-TABLE defaults (0 /
// none), exactly the state pre-0019 rows would have been in immediately after
// the schema change but before the backfill UPDATE/INSERT executed. This
// exercises the legacy-coalesce rule the migration documents: a row counts as
// redeemed if `redeemed = true` OR `redeemed_by_user_id IS NOT NULL` (the
// latter predates the boolean column).
func TestPostgresStore_LegacyCoalesceBackfill_Integration(t *testing.T) {
	ctx := context.Background()
	pool := pgtest.New(t)
	pgtest.Truncate(t, pool, "offer_code_redemptions", "offer_codes")
	store := NewPostgresStore(pool)

	// Fresh: never redeemed. redeemed=false, redeemed_by_user_id NULL.
	if _, err := pool.Exec(ctx,
		"INSERT INTO offer_codes (code, tier, duration_days, created_at, redeemed) "+
			"VALUES ('AAAAAAAAAAAA', 'Pro', 30, now(), false)"); err != nil {
		t.Fatalf("insert fresh row: %v", err)
	}
	// Redeemed, legacy shape: redeemed_by_user_id set but the boolean predates
	// it (false) — the exact "legacy coalesce" case the migration comment
	// calls out.
	if _, err := pool.Exec(ctx,
		"INSERT INTO offer_codes (code, tier, duration_days, created_at, redeemed, redeemed_by_user_id, redeemed_at) "+
			"VALUES ('BBBBBBBBBBBB', 'Pro', 30, now(), false, 'auth0|legacy', now())"); err != nil {
		t.Fatalf("insert redeemed legacy row: %v", err)
	}
	// Already-anonymised pre-migration: redeemed=true (the pre-0019 tombstone),
	// but the redeemer PII was already scrubbed (both NULL).
	if _, err := pool.Exec(ctx,
		"INSERT INTO offer_codes (code, tier, duration_days, created_at, redeemed, redeemed_by_user_id, redeemed_at) "+
			"VALUES ('CCCCCCCCCCCC', 'Pro', 30, now(), true, NULL, NULL)"); err != nil {
		t.Fatalf("insert anonymised legacy row: %v", err)
	}

	// Run the same backfill statements migration 0019 runs, mirroring its
	// legacy-coalesce predicate exactly.
	if _, err := pool.Exec(ctx,
		"UPDATE offer_codes SET redemption_count = 1 WHERE redeemed OR redeemed_by_user_id IS NOT NULL"); err != nil {
		t.Fatalf("backfill redemption_count: %v", err)
	}
	if _, err := pool.Exec(ctx,
		"INSERT INTO offer_code_redemptions (code, user_id, redeemed_at) "+
			"SELECT code, redeemed_by_user_id, redeemed_at FROM offer_codes WHERE redeemed OR redeemed_by_user_id IS NOT NULL"); err != nil {
		t.Fatalf("backfill offer_code_redemptions: %v", err)
	}

	// Fresh row: untouched, redemption_count stays 0, no child row.
	fresh, err := store.Get(ctx, "AAAAAAAAAAAA")
	if err != nil {
		t.Fatalf("Get fresh row: %v", err)
	}
	if fresh.RedemptionCount != 0 || fresh.IsFullyRedeemed() {
		t.Errorf("fresh row: got RedemptionCount=%d IsFullyRedeemed=%v, want 0/false",
			fresh.RedemptionCount, fresh.IsFullyRedeemed())
	}

	// Redeemed legacy row: backfilled to RedemptionCount=1 with a child row
	// carrying the redeemer.
	redeemed, err := store.Get(ctx, "BBBBBBBBBBBB")
	if err != nil {
		t.Fatalf("Get redeemed legacy row: %v", err)
	}
	if redeemed.RedemptionCount != 1 || !redeemed.IsFullyRedeemed() {
		t.Errorf("redeemed legacy row: got RedemptionCount=%d IsFullyRedeemed=%v, want 1/true",
			redeemed.RedemptionCount, redeemed.IsFullyRedeemed())
	}
	redemptions, err := store.RedeemedByUserID(ctx, "auth0|legacy")
	if err != nil {
		t.Fatalf("RedeemedByUserID: %v", err)
	}
	if len(redemptions) != 1 || redemptions[0].Code != "BBBBBBBBBBBB" {
		t.Fatalf("backfilled child row: got %+v, want one row for BBBBBBBBBBBB", redemptions)
	}
	// The backfilled slot must reject a fresh redemption attempt — the
	// invariant the whole migration exists to preserve.
	if err := store.RedeemWithCAS(ctx, "BBBBBBBBBBBB", "auth0|new", time.Now().UTC()); !isErr(err, ErrAlreadyRedeemed) {
		t.Fatalf("post-backfill RedeemWithCAS (redeemed legacy): got %v, want ErrAlreadyRedeemed", err)
	}

	// Already-anonymised legacy row: also backfilled to RedemptionCount=1 (its
	// redeemed=true tombstone still matches the predicate), with a tombstone
	// child row (NULL user_id/redeemed_at) — identical in shape to what
	// AnonymiseRedemptionsByUserID itself produces.
	anon, err := store.Get(ctx, "CCCCCCCCCCCC")
	if err != nil {
		t.Fatalf("Get anonymised legacy row: %v", err)
	}
	if anon.RedemptionCount != 1 || !anon.IsFullyRedeemed() {
		t.Errorf("anonymised legacy row: got RedemptionCount=%d IsFullyRedeemed=%v, want 1/true",
			anon.RedemptionCount, anon.IsFullyRedeemed())
	}
	if err := store.RedeemWithCAS(ctx, "CCCCCCCCCCCC", "auth0|new", time.Now().UTC()); !isErr(err, ErrAlreadyRedeemed) {
		t.Fatalf("post-backfill RedeemWithCAS (anonymised legacy): got %v, want ErrAlreadyRedeemed", err)
	}
}

// isErr reports whether err wraps or equals target.
func isErr(err, target error) bool {
	if err == nil {
		return false
	}
	return err == target || err.Error() == target.Error()
}
