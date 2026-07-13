//go:build integration

package polling

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/AmyDe/town-crier/api-go/internal/platform"
	"github.com/AmyDe/town-crier/api-go/internal/platform/postgres/pgtest"
)

// newPGPollStateStore returns a PostgresPollStateStore over a truncated, migrated
// test database. Integration tests MUST NOT call t.Parallel: the pgtest harness
// holds a session-level advisory lock so all integration tests in all packages
// serialise on the single docker-compose database (see pgtest.New doc).
func newPGPollStateStore(t *testing.T) *PostgresPollStateStore {
	t.Helper()
	pool := pgtest.New(t)
	pgtest.Truncate(t, pool, "poll_state", "leases")
	return NewPostgresPollStateStore(pool)
}

// newPGLeaseStoreIntegration returns a PostgresLeaseStore and its backing pool
// over a truncated, migrated test database. Both stores are returned because the
// race test needs to build two stores from the same pool.
func newPGLeaseStoreIntegration(t *testing.T) (*PostgresLeaseStore, interface {
	Exec(context.Context, string, ...any) error
}) {
	t.Helper()
	pool := pgtest.New(t)
	pgtest.Truncate(t, pool, "poll_state", "leases")
	return NewPostgresLeaseStore(pool, time.Now), nil
}

// TestPostgresPollStateStore_RoundTrip writes a fully populated PollState and
// reads it back, asserting that all fields (including the cursor) survive the
// Postgres round-trip intact.
func TestPostgresPollStateStore_RoundTrip(t *testing.T) {
	ctx := context.Background()
	store := newPGPollStateStore(t)

	lastPoll := time.Date(2026, 6, 14, 12, 0, 0, 0, time.UTC)
	hwm := time.Date(2026, 6, 13, 9, 0, 0, 0, time.UTC)
	cursor := &PollCursor{
		DifferentStart: time.Date(2026, 6, 13, 0, 0, 0, 0, time.UTC),
		NextIndex:      300,
		KnownTotal:     platform.Ptr(250),
	}

	if err := store.Save(ctx, 99, lastPoll, hwm, cursor); err != nil {
		t.Fatalf("Save: %v", err)
	}

	got, ok, err := store.Get(ctx, 99)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if !ok {
		t.Fatal("Get: expected ok=true for saved state")
	}
	if !got.LastPollTime.Equal(lastPoll) {
		t.Errorf("LastPollTime: got %v, want %v", got.LastPollTime, lastPoll)
	}
	if !got.HighWaterMark.Equal(hwm) {
		t.Errorf("HighWaterMark: got %v, want %v", got.HighWaterMark, hwm)
	}
	if got.Cursor == nil {
		t.Fatal("Cursor: expected non-nil")
	}
	if !got.Cursor.DifferentStart.Equal(cursor.DifferentStart) {
		t.Errorf("DifferentStart: got %v, want %v", got.Cursor.DifferentStart, cursor.DifferentStart)
	}
	if got.Cursor.NextIndex != 300 {
		t.Errorf("NextIndex: got %d, want 300", got.Cursor.NextIndex)
	}
	if got.Cursor.KnownTotal == nil || *got.Cursor.KnownTotal != 250 {
		t.Errorf("KnownTotal: got %v, want 250", got.Cursor.KnownTotal)
	}
}

// TestPostgresPollStateStore_RoundTrip_NilCursor confirms that a Save with a nil
// cursor stores NULL cursor columns and that a subsequent Get returns nil Cursor.
func TestPostgresPollStateStore_RoundTrip_NilCursor(t *testing.T) {
	ctx := context.Background()
	store := newPGPollStateStore(t)

	now := time.Date(2026, 6, 14, 12, 0, 0, 0, time.UTC)

	// First save with a cursor, then clear it.
	_ = store.Save(ctx, 5, now, now, &PollCursor{DifferentStart: now, NextIndex: 200})
	if err := store.Save(ctx, 5, now, now, nil); err != nil {
		t.Fatalf("Save (clear cursor): %v", err)
	}

	got, ok, err := store.Get(ctx, 5)
	if err != nil || !ok {
		t.Fatalf("Get: ok=%v err=%v", ok, err)
	}
	if got.Cursor != nil {
		t.Errorf("Cursor should be nil after clearing, got %+v", got.Cursor)
	}
}

// TestPostgresPollStateStore_MigrationBackfillsCursorNextIndexFromLegacyPage
// re-runs the additive migration's own backfill statement
// (00021_poll_state_cursor_index.sql: cursor_next_index = (cursor_next_page-1)
// * 100) against a row seeded to look exactly like a pre-migration row (only
// cursor_next_page set), proving the formula, and that Get surfaces the
// backfilled value as PollCursor.NextIndex.
func TestPostgresPollStateStore_MigrationBackfillsCursorNextIndexFromLegacyPage(t *testing.T) {
	ctx := context.Background()
	pool := pgtest.New(t)
	pgtest.Truncate(t, pool, "poll_state", "leases")

	now := time.Date(2026, 6, 14, 12, 0, 0, 0, time.UTC)
	// Seed a legacy-shaped row: only cursor_next_page is set, cursor_next_index
	// left NULL (as an ADD COLUMN would leave a pre-existing row before the
	// backfill UPDATE runs).
	if _, err := pool.Exec(ctx, `
		INSERT INTO poll_state (authority_id, last_poll_time, high_water_mark, cursor_different_start, cursor_next_page)
		VALUES ($1, $2, $2, $3, $4)`,
		300, now, now, 19,
	); err != nil {
		t.Fatalf("seed legacy row: %v", err)
	}

	// The migration's own backfill statement, verbatim.
	if _, err := pool.Exec(ctx, `
		UPDATE poll_state SET cursor_next_index = (cursor_next_page - 1) * 100
		WHERE cursor_next_page IS NOT NULL`,
	); err != nil {
		t.Fatalf("run backfill: %v", err)
	}

	var nextIndex int
	if err := pool.QueryRow(ctx, `SELECT cursor_next_index FROM poll_state WHERE authority_id = $1`, 300).Scan(&nextIndex); err != nil {
		t.Fatalf("read backfilled column: %v", err)
	}
	if nextIndex != 1800 {
		t.Errorf("cursor_next_index: got %d, want 1800 ((19-1)*100)", nextIndex)
	}

	store := NewPostgresPollStateStore(pool)
	got, ok, err := store.Get(ctx, 300)
	if err != nil || !ok {
		t.Fatalf("Get: ok=%v err=%v", ok, err)
	}
	if got.Cursor == nil || got.Cursor.NextIndex != 1800 {
		t.Errorf("Cursor: got %+v, want NextIndex=1800", got.Cursor)
	}
}

// TestPostgresPollStateStore_SaveNullsLegacyCursorNextPageColumn proves that
// Save always migrates a row forward: even when a legacy cursor_next_page
// value is already present (left by an old-code writer during the
// deploy-overlap window), a subsequent Save nulls it out and writes
// cursor_next_index instead.
func TestPostgresPollStateStore_SaveNullsLegacyCursorNextPageColumn(t *testing.T) {
	ctx := context.Background()
	pool := pgtest.New(t)
	pgtest.Truncate(t, pool, "poll_state", "leases")

	now := time.Date(2026, 6, 14, 12, 0, 0, 0, time.UTC)
	if _, err := pool.Exec(ctx, `
		INSERT INTO poll_state (authority_id, last_poll_time, high_water_mark, cursor_different_start, cursor_next_page)
		VALUES ($1, $2, $2, $3, $4)`,
		400, now, now, 5,
	); err != nil {
		t.Fatalf("seed legacy row: %v", err)
	}

	store := NewPostgresPollStateStore(pool)
	if err := store.Save(ctx, 400, now, now, &PollCursor{DifferentStart: now, NextIndex: 777}); err != nil {
		t.Fatalf("Save: %v", err)
	}

	var (
		nextPage  *int
		nextIndex *int
	)
	if err := pool.QueryRow(ctx, `SELECT cursor_next_page, cursor_next_index FROM poll_state WHERE authority_id = $1`, 400).
		Scan(&nextPage, &nextIndex); err != nil {
		t.Fatalf("read row: %v", err)
	}
	if nextPage != nil {
		t.Errorf("cursor_next_page: got %v, want NULL after Save", *nextPage)
	}
	if nextIndex == nil || *nextIndex != 777 {
		t.Errorf("cursor_next_index: got %v, want 777", nextIndex)
	}
}

// TestPostgresPollStateStore_GetMissing confirms that a miss returns ok=false
// with no error — the normal "never polled" state.
func TestPostgresPollStateStore_GetMissing(t *testing.T) {
	ctx := context.Background()
	store := newPGPollStateStore(t)

	_, ok, err := store.Get(ctx, 9999)
	if err != nil {
		t.Fatalf("Get on miss: %v", err)
	}
	if ok {
		t.Error("Get on miss: expected ok=false")
	}
}

// TestPostgresPollStateStore_GetLeastRecentlyPolled_Ordering proves the SQL
// ordering invariant: never-polled authorities (no row in poll_state) appear
// first, then authorities sorted by last_poll_time ascending. The never-polled
// count must also be accurate.
func TestPostgresPollStateStore_GetLeastRecentlyPolled_Ordering(t *testing.T) {
	ctx := context.Background()
	store := newPGPollStateStore(t)

	t0 := time.Date(2026, 6, 14, 10, 0, 0, 0, time.UTC)
	// Authority 100: polled most recently.
	if err := store.Save(ctx, 100, t0.Add(2*time.Hour), t0, nil); err != nil {
		t.Fatalf("Save 100: %v", err)
	}
	// Authority 200: polled longest ago.
	if err := store.Save(ctx, 200, t0, t0, nil); err != nil {
		t.Fatalf("Save 200: %v", err)
	}
	// Authority 300: never polled (no row).

	res, err := store.GetLeastRecentlyPolled(ctx, []int{100, 200, 300})
	if err != nil {
		t.Fatalf("GetLeastRecentlyPolled: %v", err)
	}

	want := []int{300, 200, 100}
	if len(res.AuthorityIDs) != len(want) {
		t.Fatalf("AuthorityIDs: got %v, want %v", res.AuthorityIDs, want)
	}
	for i, id := range want {
		if res.AuthorityIDs[i] != id {
			t.Errorf("order[%d]: got %d, want %d (full=%v)", i, res.AuthorityIDs[i], id, res.AuthorityIDs)
		}
	}
	if res.NeverPolledCount != 1 {
		t.Errorf("NeverPolledCount: got %d, want 1", res.NeverPolledCount)
	}
}

// TestPostgresPollStateStore_GetLeastRecentlyPolled_AllNeverPolled confirms
// correct behaviour when none of the candidates have a poll_state row.
func TestPostgresPollStateStore_GetLeastRecentlyPolled_AllNeverPolled(t *testing.T) {
	ctx := context.Background()
	store := newPGPollStateStore(t)

	res, err := store.GetLeastRecentlyPolled(ctx, []int{1, 2, 3})
	if err != nil {
		t.Fatalf("GetLeastRecentlyPolled: %v", err)
	}
	if res.NeverPolledCount != 3 {
		t.Errorf("NeverPolledCount: got %d, want 3", res.NeverPolledCount)
	}
	if len(res.AuthorityIDs) != 3 {
		t.Errorf("AuthorityIDs: got %v, want len=3", res.AuthorityIDs)
	}
}

// newPGLeaseStore returns a PostgresLeaseStore over a truncated test database.
func newPGLeaseStore2(t *testing.T) *PostgresLeaseStore {
	t.Helper()
	pool := pgtest.New(t)
	pgtest.Truncate(t, pool, "poll_state", "leases")
	return NewPostgresLeaseStore(pool, time.Now)
}

// TestPostgresLeaseStore_AcquireCreatesWhenAbsent confirms that TryAcquire
// inserts the lease row and reports Acquired=true when no row exists.
func TestPostgresLeaseStore_AcquireCreatesWhenAbsent(t *testing.T) {
	ctx := context.Background()
	store := newPGLeaseStore2(t)

	res, err := store.TryAcquire(ctx, 4*time.Minute)
	if err != nil {
		t.Fatalf("TryAcquire: %v", err)
	}
	if !res.Acquired {
		t.Fatalf("expected Acquired=true, got %+v", res)
	}
	if res.Handle.ETag == "" {
		t.Error("expected non-empty Handle.ETag")
	}
}

// TestPostgresLeaseStore_AcquireHeldWhenLive confirms that a second TryAcquire
// while the first lease is still live reports Held=true.
func TestPostgresLeaseStore_AcquireHeldWhenLive(t *testing.T) {
	ctx := context.Background()
	store := newPGLeaseStore2(t)

	res1, err := store.TryAcquire(ctx, 4*time.Minute)
	if err != nil || !res1.Acquired {
		t.Fatalf("first TryAcquire: acquired=%v err=%v", res1.Acquired, err)
	}

	store2 := NewPostgresLeaseStore(store.db, time.Now)
	res2, err := store2.TryAcquire(ctx, 4*time.Minute)
	if err != nil {
		t.Fatalf("second TryAcquire: %v", err)
	}
	if res2.Acquired {
		t.Error("second acquire must not succeed while lease is live")
	}
	if !res2.Held {
		t.Error("expected Held=true on second acquire")
	}
}

// TestPostgresLeaseStore_AcquireReplacesExpiredLease confirms that a lease
// whose expires_at has elapsed can be taken over by a new caller.
func TestPostgresLeaseStore_AcquireReplacesExpiredLease(t *testing.T) {
	ctx := context.Background()

	// Build a store with a past clock so its lease is immediately expired.
	past := time.Now().UTC().Add(-10 * time.Minute)
	expiredClock := func() time.Time { return past }
	pool := pgtest.New(t)
	pgtest.Truncate(t, pool, "poll_state", "leases")

	storeA := NewPostgresLeaseStore(pool, expiredClock)
	res1, err := storeA.TryAcquire(ctx, 1*time.Second) // TTL in the past
	if err != nil || !res1.Acquired {
		t.Fatalf("storeA TryAcquire: acquired=%v err=%v", res1.Acquired, err)
	}

	// storeB uses the real clock and should see an expired row.
	storeB := NewPostgresLeaseStore(pool, time.Now)
	res2, err := storeB.TryAcquire(ctx, 4*time.Minute)
	if err != nil {
		t.Fatalf("storeB TryAcquire: %v", err)
	}
	if !res2.Acquired {
		t.Errorf("storeB should acquire the expired lease, got %+v", res2)
	}
}

// TestPostgresLeaseStore_ReleaseOutcomes covers the three expected release
// outcomes: LeaseReleased (normal), LeaseAlreadyGone (row absent),
// LeasePreconditionFailed (different holder holds the row).
func TestPostgresLeaseStore_ReleaseOutcomes(t *testing.T) {
	ctx := context.Background()

	t.Run("released", func(t *testing.T) {
		store := newPGLeaseStore2(t)
		res, _ := store.TryAcquire(ctx, 4*time.Minute)
		outcome := store.Release(ctx, res.Handle)
		if outcome != LeaseReleased {
			t.Errorf("Release: got %v, want LeaseReleased", outcome)
		}
	})

	t.Run("already gone", func(t *testing.T) {
		store := newPGLeaseStore2(t)
		// Never acquired, so there is no row to delete.
		outcome := store.Release(ctx, LeaseHandle{ETag: "nonexistent-holder"})
		if outcome != LeaseAlreadyGone {
			t.Errorf("Release: got %v, want LeaseAlreadyGone", outcome)
		}
	})

	t.Run("precondition failed", func(t *testing.T) {
		pool := pgtest.New(t)
		pgtest.Truncate(t, pool, "poll_state", "leases")

		storeA := NewPostgresLeaseStore(pool, time.Now)
		storeA.TryAcquire(ctx, 4*time.Minute) //nolint:errcheck // result not needed

		// storeB tries to release with its own (non-matching) holder id.
		storeB := NewPostgresLeaseStore(pool, time.Now)
		outcome := storeB.Release(ctx, LeaseHandle{ETag: storeB.holderID})
		if outcome != LeasePreconditionFailed {
			t.Errorf("Release: got %v, want LeasePreconditionFailed", outcome)
		}
	})
}

// TestPostgresLeaseStore_Confirm covers the three CAS outcomes the orchestrator's
// lease-confirmed publish (GH#938 PR1) depends on against a real database: still
// held by us and live -> true with the expiry extended; expired -> false; held by
// a different holder -> false. This is additive real-DB coverage (ADR 0032) for
// SQL behaviour the fake querier in TestPostgresLeaseStore_ConfirmCAS cannot
// honestly prove — that the UPDATE ... WHERE ... RETURNING actually extends
// expires_at in Postgres.
func TestPostgresLeaseStore_Confirm(t *testing.T) {
	ctx := context.Background()

	t.Run("held by us and live: confirms and extends expiry", func(t *testing.T) {
		store := newPGLeaseStore2(t)
		res, err := store.TryAcquire(ctx, 1*time.Minute)
		if err != nil || !res.Acquired {
			t.Fatalf("TryAcquire: acquired=%v err=%v", res.Acquired, err)
		}

		if ok := store.Confirm(ctx, res.Handle, 10*time.Minute); !ok {
			t.Fatal("Confirm: got false, want true (live lease held by us)")
		}

		// The extend must have taken effect: a peer's TryAcquire, evaluated against
		// a clock only 1 minute in the future (the original TTL would already have
		// expired by then absent the extend), must still see it as live.
		peer := NewPostgresLeaseStore(store.db, func() time.Time { return time.Now().UTC().Add(1 * time.Minute) })
		peerRes, err := peer.TryAcquire(ctx, 1*time.Minute)
		if err != nil {
			t.Fatalf("peer TryAcquire: %v", err)
		}
		if peerRes.Acquired {
			t.Error("peer acquired the lease; Confirm's extend did not take effect")
		}
		if !peerRes.Held {
			t.Error("expected peer to see the lease as still held (extended)")
		}
	})

	t.Run("expired: does not confirm", func(t *testing.T) {
		past := time.Now().UTC().Add(-10 * time.Minute)
		expiredClock := func() time.Time { return past }
		pool := pgtest.New(t)
		pgtest.Truncate(t, pool, "poll_state", "leases")

		store := NewPostgresLeaseStore(pool, expiredClock)
		res, err := store.TryAcquire(ctx, 1*time.Second) // TTL already in the past
		if err != nil || !res.Acquired {
			t.Fatalf("TryAcquire: acquired=%v err=%v", res.Acquired, err)
		}

		// Confirm from the real clock: the row's expires_at is long past, so the
		// liveness predicate (expires_at > now) is false regardless of holder_id.
		realClockStore := NewPostgresLeaseStore(pool, time.Now)
		if ok := realClockStore.Confirm(ctx, res.Handle, 10*time.Minute); ok {
			t.Error("Confirm: got true, want false (lease already expired)")
		}
	})

	t.Run("held by a different holder: does not confirm", func(t *testing.T) {
		store := newPGLeaseStore2(t)
		res, err := store.TryAcquire(ctx, 4*time.Minute)
		if err != nil || !res.Acquired {
			t.Fatalf("TryAcquire: acquired=%v err=%v", res.Acquired, err)
		}

		peer := NewPostgresLeaseStore(store.db, time.Now)
		if ok := peer.Confirm(ctx, LeaseHandle{ETag: peer.holderID}, 10*time.Minute); ok {
			t.Error("Confirm: got true, want false (handle belongs to a different holder)")
		}
	})
}

// TestPostgresLeaseStore_RaceProof is the required acceptance-criterion test:
// two concurrent goroutines both call TryAcquire; exactly one must win
// (Acquired=true) and the other must be told the lease is unavailable
// (Held=true). This proves the atomic INSERT ... ON CONFLICT ... WHERE behaviour
// gates concurrent poll cycles at parity with the Cosmos etag-CAS model.
func TestPostgresLeaseStore_RaceProof(t *testing.T) {
	ctx := context.Background()
	pool := pgtest.New(t)
	pgtest.Truncate(t, pool, "poll_state", "leases")

	storeA := NewPostgresLeaseStore(pool, time.Now)
	storeB := NewPostgresLeaseStore(pool, time.Now)

	type result struct {
		res LeaseAcquireResult
		err error
	}
	ch := make(chan result, 2)

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		r, e := storeA.TryAcquire(ctx, 4*time.Minute)
		ch <- result{r, e}
	}()
	go func() {
		defer wg.Done()
		r, e := storeB.TryAcquire(ctx, 4*time.Minute)
		ch <- result{r, e}
	}()
	wg.Wait()
	close(ch)

	var acquired, held int
	for r := range ch {
		if r.err != nil {
			t.Fatalf("TryAcquire returned hard error: %v", r.err)
		}
		if r.res.Acquired {
			acquired++
		}
		if r.res.Held {
			held++
		}
	}

	if acquired != 1 {
		t.Errorf("race proof: expected exactly 1 acquired, got %d", acquired)
	}
	if held != 1 {
		t.Errorf("race proof: expected exactly 1 held, got %d", held)
	}
}
