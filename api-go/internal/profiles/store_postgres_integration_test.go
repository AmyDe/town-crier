//go:build integration

package profiles

import (
	"context"
	"errors"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/AmyDe/town-crier/api-go/internal/platform"
	"github.com/AmyDe/town-crier/api-go/internal/platform/postgres/pgtest"
)

// newUserPGStore returns a PostgresStore and PostgresAdminStore backed by a
// freshly truncated test database. Integration tests must NOT call t.Parallel:
// pgtest.New holds a session-level advisory lock for mutual exclusion.
func newUserPGStore(t *testing.T) (*PostgresStore, *PostgresAdminStore) {
	t.Helper()
	pool := pgtest.New(t)
	pgtest.Truncate(t, pool, "users")
	return NewPostgresStore(pool), NewPostgresAdminStore(pool)
}

// pgProfile builds a deterministic UserProfile for test seeding. It creates a
// profile with a fixed active-at time so dormant/lapsed assertions are stable.
func pgProfile(t *testing.T, userID, email string) *UserProfile {
	t.Helper()
	now := time.Date(2026, 6, 26, 12, 0, 0, 0, time.UTC)
	p, err := NewProfile(userID, email, now)
	if err != nil {
		t.Fatalf("NewProfile(%s): %v", userID, err)
	}
	return p
}

// assertProfileEqual compares two profiles field-by-field, zeroing timestamps
// to microsecond precision (Postgres timestamptz truncates sub-µs).
func assertProfileEqual(t *testing.T, got, want *UserProfile) {
	t.Helper()
	// Round trip: Postgres timestamptz is µs precision; Go time.Time may carry ns.
	g, w := *got, *want
	g.LastActiveAt = got.LastActiveAt.UTC().Truncate(time.Microsecond)
	w.LastActiveAt = want.LastActiveAt.UTC().Truncate(time.Microsecond)
	if !reflect.DeepEqual(g, w) {
		t.Errorf("profile mismatch:\n got = %+v\nwant = %+v", g, w)
	}
}

// TestPostgresStore_SaveGetRoundTrip writes a profile and reads it back,
// asserting all fields survive the Postgres round-trip unchanged.
func TestPostgresStore_SaveGetRoundTrip(t *testing.T) {
	store, _ := newUserPGStore(t)
	ctx := context.Background()

	p := pgProfile(t, "auth0|rt1", "alice@example.com")
	one := 1
	p.WatchZoneCount = &one
	p.ZonePreferences = map[string]ZonePreferences{
		"zone-abc": {NewApplicationPush: true, NewApplicationEmail: false, DecisionPush: true, DecisionEmail: false},
	}

	if err := store.Save(ctx, p); err != nil {
		t.Fatalf("Save: %v", err)
	}

	got, err := store.Get(ctx, p.UserID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	assertProfileEqual(t, got, p)
}

// TestPostgresStore_Get_Miss confirms a missing profile returns ErrNotFound.
func TestPostgresStore_Get_Miss(t *testing.T) {
	store, _ := newUserPGStore(t)

	_, err := store.Get(context.Background(), "auth0|nobody")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("Get miss: got %v, want ErrNotFound", err)
	}
}

// TestPostgresStore_Delete_Miss confirms that deleting a non-existent profile
// returns ErrNotFound.
func TestPostgresStore_Delete_Miss(t *testing.T) {
	store, _ := newUserPGStore(t)

	err := store.Delete(context.Background(), "auth0|nobody")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("Delete miss: got %v, want ErrNotFound", err)
	}
}

// TestPostgresStore_Delete_Removes verifies that a saved profile is gone after
// Delete and that Get then returns ErrNotFound.
func TestPostgresStore_Delete_Removes(t *testing.T) {
	store, _ := newUserPGStore(t)
	ctx := context.Background()

	p := pgProfile(t, "auth0|del1", "bob@example.com")
	if err := store.Save(ctx, p); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if err := store.Delete(ctx, p.UserID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := store.Get(ctx, p.UserID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("post-Delete Get: got %v, want ErrNotFound", err)
	}
}

// TestPostgresStore_GetWithETag_Miss confirms that GetWithETag for a missing
// profile returns (nil, "", nil) — the watch-zone quota handler treats absence
// as a benign no-op.
func TestPostgresStore_GetWithETag_Miss(t *testing.T) {
	store, _ := newUserPGStore(t)

	got, etag, err := store.GetWithETag(context.Background(), "auth0|nobody")
	if err != nil {
		t.Fatalf("GetWithETag miss: got error %v, want nil", err)
	}
	if got != nil {
		t.Errorf("GetWithETag miss: profile = %v, want nil", got)
	}
	if etag != "" {
		t.Errorf("GetWithETag miss: etag = %q, want empty", etag)
	}
}

// TestPostgresStore_GetWithETag_Present verifies that GetWithETag returns the
// profile and a non-empty, parseable etag for an existing profile. A new row
// always starts at version 0 so the etag should be "0".
func TestPostgresStore_GetWithETag_Present(t *testing.T) {
	store, _ := newUserPGStore(t)
	ctx := context.Background()

	p := pgProfile(t, "auth0|etag1", "carol@example.com")
	if err := store.Save(ctx, p); err != nil {
		t.Fatalf("Save: %v", err)
	}

	got, etag, err := store.GetWithETag(ctx, p.UserID)
	if err != nil {
		t.Fatalf("GetWithETag: %v", err)
	}
	if got == nil {
		t.Fatal("GetWithETag: profile is nil")
	}
	if etag == "" {
		t.Fatal("GetWithETag: etag is empty")
	}
	// New row starts at version 0.
	if etag != "0" {
		t.Errorf("GetWithETag: etag = %q, want \"0\"", etag)
	}
}

// TestPostgresStore_UpdateZoneCountWithCAS_Success verifies the happy path:
// a matching version succeeds and the version increments (etag changes).
func TestPostgresStore_UpdateZoneCountWithCAS_Success(t *testing.T) {
	store, _ := newUserPGStore(t)
	ctx := context.Background()

	p := pgProfile(t, "auth0|cas1", "dave@example.com")
	if err := store.Save(ctx, p); err != nil {
		t.Fatalf("Save: %v", err)
	}

	_, etag, err := store.GetWithETag(ctx, p.UserID)
	if err != nil {
		t.Fatalf("GetWithETag: %v", err)
	}

	one := 1
	p.WatchZoneCount = &one
	if err := store.UpdateZoneCountWithCAS(ctx, p.UserID, p, etag); err != nil {
		t.Fatalf("UpdateZoneCountWithCAS: %v", err)
	}

	_, newEtag, err := store.GetWithETag(ctx, p.UserID)
	if err != nil {
		t.Fatalf("GetWithETag after CAS: %v", err)
	}
	if newEtag == etag {
		t.Errorf("etag unchanged after CAS success: still %q", etag)
	}
}

// TestPostgresStore_CASRaceProof is the required concurrency proof: two
// goroutines each attempt UpdateZoneCountWithCAS with the same etag; exactly
// one succeeds and the other gets ErrCASPreconditionFailed. This test directly
// exercises the optimistic-lock mechanism that the watch-zone quota handler
// relies on to prevent double-counting.
func TestPostgresStore_CASRaceProof(t *testing.T) {
	store, _ := newUserPGStore(t)
	ctx := context.Background()

	p := pgProfile(t, "auth0|race1", "eve@example.com")
	if err := store.Save(ctx, p); err != nil {
		t.Fatalf("Save: %v", err)
	}
	_, etag, err := store.GetWithETag(ctx, p.UserID)
	if err != nil {
		t.Fatalf("GetWithETag: %v", err)
	}

	// Both goroutines fire UpdateZoneCountWithCAS concurrently with the same etag.
	// Exactly one must succeed; the other must return ErrCASPreconditionFailed.
	var (
		wg        sync.WaitGroup
		mu        sync.Mutex
		successes int
		casErrs   int
		otherErrs []error
	)

	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			one := 1
			cp := *p
			cp.WatchZoneCount = &one
			err := store.UpdateZoneCountWithCAS(ctx, p.UserID, &cp, etag)
			mu.Lock()
			defer mu.Unlock()
			switch {
			case err == nil:
				successes++
			case errors.Is(err, platform.ErrCASPreconditionFailed):
				casErrs++
			default:
				otherErrs = append(otherErrs, err)
			}
		}()
	}
	wg.Wait()

	if len(otherErrs) > 0 {
		t.Fatalf("unexpected errors from CAS race: %v", otherErrs)
	}
	if successes != 1 {
		t.Errorf("CAS race: %d successes, want exactly 1", successes)
	}
	if casErrs != 1 {
		t.Errorf("CAS race: %d precondition failures, want exactly 1", casErrs)
	}
}

// TestPostgresStore_LegacyCoalesceRoundTrip verifies the opt-in default: when
// email_digest_enabled / saved_decision_* are inserted as NULL (legacy profile
// written before these fields existed), they read back as true.
func TestPostgresStore_LegacyCoalesceRoundTrip(t *testing.T) {
	store, _ := newUserPGStore(t)
	ctx := context.Background()

	// Directly INSERT a row with NULL nullable-bool columns, bypassing Save
	// (which always writes non-null booleans). This exercises the coalesceTrue
	// logic in scanUserRow for real rows — the precise scenario DecodeDocument
	// handles when backfilling legacy Cosmos documents.
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	_, err := store.db.Exec(ctx, `
		INSERT INTO users (
			user_id, push_enabled, digest_day,
			email_digest_enabled, saved_decision_push, saved_decision_email,
			zone_preferences, tier, last_active_at, last_active_at_epoch
		) VALUES (
			$1, true, 1,
			NULL, NULL, NULL,
			'{}', 'Free', $2, $3
		)`,
		"auth0|legacy1", now, now.UnixMilli(),
	)
	if err != nil {
		t.Fatalf("INSERT legacy row: %v", err)
	}

	got, err := store.Get(ctx, "auth0|legacy1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if !got.Preferences.EmailDigestEnabled {
		t.Error("EmailDigestEnabled: got false, want true (NULL should coalesce to true)")
	}
	if !got.Preferences.SavedDecisionPush {
		t.Error("SavedDecisionPush: got false, want true (NULL should coalesce to true)")
	}
	if !got.Preferences.SavedDecisionEmail {
		t.Error("SavedDecisionEmail: got false, want true (NULL should coalesce to true)")
	}
}

// ---- Admin store integration tests ----

// TestPostgresAdminStore_GetByEmail verifies point-read by email address and
// ErrNotFound for a non-existent email.
func TestPostgresAdminStore_GetByEmail(t *testing.T) {
	store, admin := newUserPGStore(t)
	ctx := context.Background()

	p := pgProfile(t, "auth0|adm1", "frank@example.com")
	if err := store.Save(ctx, p); err != nil {
		t.Fatalf("Save: %v", err)
	}

	got, err := admin.GetByEmail(ctx, "frank@example.com")
	if err != nil {
		t.Fatalf("GetByEmail: %v", err)
	}
	if got.UserID != p.UserID {
		t.Errorf("GetByEmail: got userID %q, want %q", got.UserID, p.UserID)
	}

	_, err = admin.GetByEmail(ctx, "nobody@example.com")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("GetByEmail miss: got %v, want ErrNotFound", err)
	}
}

// TestPostgresAdminStore_GetByOriginalTransactionID verifies lookup by Apple
// transaction id and ErrNotFound for a missing id.
func TestPostgresAdminStore_GetByOriginalTransactionID(t *testing.T) {
	pool := pgtest.New(t)
	pgtest.Truncate(t, pool, "users")
	store := NewPostgresStore(pool)
	admin := NewPostgresAdminStore(pool)
	ctx := context.Background()

	p := pgProfile(t, "auth0|adm2", "grace@example.com")
	txID := "orig-tx-999"
	p.OriginalTransactionID = &txID
	if err := store.Save(ctx, p); err != nil {
		t.Fatalf("Save: %v", err)
	}

	got, err := admin.GetByOriginalTransactionID(ctx, txID)
	if err != nil {
		t.Fatalf("GetByOriginalTransactionID: %v", err)
	}
	if got.UserID != p.UserID {
		t.Errorf("GetByOriginalTransactionID: got userID %q, want %q", got.UserID, p.UserID)
	}

	_, err = admin.GetByOriginalTransactionID(ctx, "no-such-tx")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("GetByOriginalTransactionID miss: got %v, want ErrNotFound", err)
	}
}

// TestPostgresAdminStore_ByDigestDay verifies that only profiles whose
// DigestDay matches are returned.
func TestPostgresAdminStore_ByDigestDay(t *testing.T) {
	pool := pgtest.New(t)
	pgtest.Truncate(t, pool, "users")
	store := NewPostgresStore(pool)
	admin := NewPostgresAdminStore(pool)
	ctx := context.Background()

	// Three profiles: two on Monday, one on Tuesday.
	for _, id := range []string{"auth0|adm3a", "auth0|adm3b"} {
		p := pgProfile(t, id, id+"@example.com")
		p.Preferences.DigestDay = time.Monday
		if err := store.Save(ctx, p); err != nil {
			t.Fatalf("Save %s: %v", id, err)
		}
	}
	pTue := pgProfile(t, "auth0|adm3c", "adm3c@example.com")
	pTue.Preferences.DigestDay = time.Tuesday
	if err := store.Save(ctx, pTue); err != nil {
		t.Fatalf("Save Tuesday: %v", err)
	}

	monProfiles, err := admin.ByDigestDay(ctx, time.Monday)
	if err != nil {
		t.Fatalf("ByDigestDay Monday: %v", err)
	}
	if len(monProfiles) != 2 {
		t.Errorf("ByDigestDay Monday: got %d, want 2", len(monProfiles))
	}

	tueProfiles, err := admin.ByDigestDay(ctx, time.Tuesday)
	if err != nil {
		t.Fatalf("ByDigestDay Tuesday: %v", err)
	}
	if len(tueProfiles) != 1 {
		t.Errorf("ByDigestDay Tuesday: got %d, want 1", len(tueProfiles))
	}
}

// TestPostgresAdminStore_Dormant verifies that profiles last active before the
// cutoff are returned and profiles active at or after are excluded.
func TestPostgresAdminStore_Dormant(t *testing.T) {
	pool := pgtest.New(t)
	pgtest.Truncate(t, pool, "users")
	store := NewPostgresStore(pool)
	admin := NewPostgresAdminStore(pool)
	ctx := context.Background()

	cutoff := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	// Dormant: last active before cutoff.
	pDormant := pgProfile(t, "auth0|adm4a", "dorm@example.com")
	pDormant.LastActiveAt = cutoff.Add(-24 * time.Hour)
	if err := store.Save(ctx, pDormant); err != nil {
		t.Fatalf("Save dormant: %v", err)
	}

	// Active: last active after cutoff.
	pActive := pgProfile(t, "auth0|adm4b", "active@example.com")
	pActive.LastActiveAt = cutoff.Add(24 * time.Hour)
	if err := store.Save(ctx, pActive); err != nil {
		t.Fatalf("Save active: %v", err)
	}

	dormant, err := admin.Dormant(ctx, cutoff)
	if err != nil {
		t.Fatalf("Dormant: %v", err)
	}
	if len(dormant) != 1 {
		t.Errorf("Dormant: got %d profiles, want 1", len(dormant))
	}
	if len(dormant) > 0 && dormant[0].UserID != pDormant.UserID {
		t.Errorf("Dormant: got userID %q, want %q", dormant[0].UserID, pDormant.UserID)
	}
}

// TestPostgresAdminStore_LapsedPaid verifies that profiles with a paid tier but
// a past subscription expiry appear in LapsedPaid, while active paid and free
// tier profiles do not.
func TestPostgresAdminStore_LapsedPaid(t *testing.T) {
	pool := pgtest.New(t)
	pgtest.Truncate(t, pool, "users")
	store := NewPostgresStore(pool)
	admin := NewPostgresAdminStore(pool)
	ctx := context.Background()

	now := time.Date(2026, 6, 26, 12, 0, 0, 0, time.UTC)

	// Lapsed: Personal tier, expired 30 days ago.
	pLapsed := pgProfile(t, "auth0|adm5a", "lapsed@example.com")
	pLapsed.Tier = TierPersonal
	exp := now.Add(-30 * 24 * time.Hour)
	pLapsed.SubscriptionExpiry = &exp
	if err := store.Save(ctx, pLapsed); err != nil {
		t.Fatalf("Save lapsed: %v", err)
	}

	// Active paid: Personal tier, expiry in the future.
	pPaid := pgProfile(t, "auth0|adm5b", "paid@example.com")
	pPaid.Tier = TierPersonal
	future := now.Add(30 * 24 * time.Hour)
	pPaid.SubscriptionExpiry = &future
	if err := store.Save(ctx, pPaid); err != nil {
		t.Fatalf("Save paid: %v", err)
	}

	// Free tier: should never appear.
	pFree := pgProfile(t, "auth0|adm5c", "free@example.com")
	if err := store.Save(ctx, pFree); err != nil {
		t.Fatalf("Save free: %v", err)
	}

	lapsed, err := admin.LapsedPaid(ctx, now)
	if err != nil {
		t.Fatalf("LapsedPaid: %v", err)
	}
	if len(lapsed) != 1 {
		t.Errorf("LapsedPaid: got %d profiles, want 1", len(lapsed))
	}
	if len(lapsed) > 0 && lapsed[0].UserID != pLapsed.UserID {
		t.Errorf("LapsedPaid: got userID %q, want %q", lapsed[0].UserID, pLapsed.UserID)
	}
}

// TestPostgresAdminStore_Save verifies that the admin Save method upserts a
// profile (same contract as PostgresStore.Save but accessible from the admin
// surface — used by the subscription sweep).
func TestPostgresAdminStore_Save(t *testing.T) {
	pool := pgtest.New(t)
	pgtest.Truncate(t, pool, "users")
	store := NewPostgresStore(pool)
	admin := NewPostgresAdminStore(pool)
	ctx := context.Background()

	p := pgProfile(t, "auth0|adm6", "hannah@example.com")
	if err := admin.Save(ctx, p); err != nil {
		t.Fatalf("admin Save: %v", err)
	}
	got, err := store.Get(ctx, p.UserID)
	if err != nil {
		t.Fatalf("Get after admin Save: %v", err)
	}
	if got.UserID != p.UserID {
		t.Errorf("admin Save: got userID %q, want %q", got.UserID, p.UserID)
	}
}

// TestPostgresAdminStore_List_Pagination verifies that List pages correctly
// through profiles ordered by user_id using the keyset cursor, and that an
// email filter narrows results.
func TestPostgresAdminStore_List_Pagination(t *testing.T) {
	pool := pgtest.New(t)
	pgtest.Truncate(t, pool, "users")
	store := NewPostgresStore(pool)
	admin := NewPostgresAdminStore(pool)
	ctx := context.Background()

	// Seed 5 profiles with deterministic, ordered IDs so pagination order is stable.
	for i := 1; i <= 5; i++ {
		id := "auth0|list" + string(rune('0'+i))
		email := id + "@example.com"
		if err := store.Save(ctx, pgProfile(t, id, email)); err != nil {
			t.Fatalf("Save %s: %v", id, err)
		}
	}

	// Page 1: first 3.
	page1, err := admin.List(ctx, "", 3, "")
	if err != nil {
		t.Fatalf("List page 1: %v", err)
	}
	if len(page1.Profiles) != 3 {
		t.Errorf("page 1: got %d profiles, want 3", len(page1.Profiles))
	}
	if page1.ContinuationToken == "" {
		t.Error("page 1: expected continuation token, got empty")
	}

	// Page 2: next 2 using token.
	page2, err := admin.List(ctx, "", 3, page1.ContinuationToken)
	if err != nil {
		t.Fatalf("List page 2: %v", err)
	}
	if len(page2.Profiles) != 2 {
		t.Errorf("page 2: got %d profiles, want 2", len(page2.Profiles))
	}
	if page2.ContinuationToken != "" {
		t.Errorf("page 2: expected empty token (last page), got %q", page2.ContinuationToken)
	}

	// Email filter: only profile 3 matches "list3".
	filtered, err := admin.List(ctx, "list3", 10, "")
	if err != nil {
		t.Fatalf("List filtered: %v", err)
	}
	if len(filtered.Profiles) != 1 {
		t.Errorf("filtered: got %d profiles, want 1", len(filtered.Profiles))
	}
}
