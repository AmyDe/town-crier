package polling

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// Compile-time check: the Postgres lease store satisfies LeaseAccess.
var _ LeaseAccess = (*PostgresLeaseStore)(nil)

// fakeLeaseQuerier is a hand-written double for the lease store's querier that
// covers the error and rows-affected branches without a real database.
type fakeLeaseQuerier struct {
	// TryAcquire path: QueryRow returns a single row or pgx.ErrNoRows.
	acquireRowErr error

	// Release path: Exec returns the configured tag and error.
	releaseExecTag pgconn.CommandTag
	releaseExecErr error

	// Second query in Release (exists check): QueryRow returns a count.
	existsCount int
	existsErr   error
}

// fakeLeaseRow implements pgx.Row for the TryAcquire RETURNING path.
// When err is pgx.ErrNoRows, Scan signals "held". Any other non-nil err is
// transient. A nil err means the INSERT/UPDATE succeeded and we scan holderID.
type fakeLeaseRow struct {
	err      error
	holderID string
}

func (r *fakeLeaseRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	if len(dest) != 1 {
		return errors.New("fakeLeaseRow: expected 1 scan dest")
	}
	*dest[0].(*string) = r.holderID
	return nil
}

// fakeExistsRow implements pgx.Row for the lease-exists check in Release.
type fakeExistsRow struct {
	count int
	err   error
}

func (r *fakeExistsRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	if len(dest) != 1 {
		return errors.New("fakeExistsRow: expected 1 scan dest")
	}
	*dest[0].(*int) = r.count
	return nil
}

// queryCallN tracks which QueryRow call is being made (first = TryAcquire,
// second = exists-check in Release). The lease store makes at most two
// QueryRow calls within a single operation.
func (f *fakeLeaseQuerier) queryCallN() *int {
	n := 0
	return &n
}

var _ = (*fakeLeaseQuerier)(nil) // ensure fakeLeaseQuerier is used

func (f *fakeLeaseQuerier) Exec(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
	return f.releaseExecTag, f.releaseExecErr
}

func (f *fakeLeaseQuerier) Query(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
	return &fakePollRows{}, nil
}

// QueryRow routes to TryAcquire row or exists-check row based on call count.
// We distinguish them by whether acquireRowErr is sentinel pgx.ErrNoRows or the
// caller has already set releaseExecTag (meaning we're in a Release path).
func (f *fakeLeaseQuerier) QueryRow(_ context.Context, _ string, _ ...any) pgx.Row {
	// If the exec tag was set (Release path second query): return the exists row.
	// Otherwise return the TryAcquire row.
	if f.releaseExecTag.RowsAffected() > 0 {
		// Release already succeeded; no second QueryRow needed.
		return &fakeExistsRow{count: 0}
	}
	if f.releaseExecErr == nil && f.acquireRowErr == nil {
		// Default: TryAcquire succeeded.
		return &fakeLeaseRow{holderID: "test-holder"}
	}
	if f.acquireRowErr != nil {
		return &fakeLeaseRow{err: f.acquireRowErr}
	}
	return &fakeExistsRow{count: f.existsCount, err: f.existsErr}
}

// newPGLeaseStore returns a PostgresLeaseStore with a fixed clock for tests.
func newPGLeaseStore(q querier) *PostgresLeaseStore {
	clock := func() time.Time { return time.Date(2026, 6, 14, 12, 0, 0, 0, time.UTC) }
	return NewPostgresLeaseStore(q, clock)
}

// fakeConfirmQuerier is a hand-written double isolated to the Confirm CAS's
// single QueryRow round-trip. Confirm never calls Exec or Query, so those panic
// to catch a wiring mistake rather than silently doing the wrong thing; this
// keeps Confirm's test isolated from fakeLeaseQuerier's TryAcquire/Release
// call-count routing.
type fakeConfirmQuerier struct {
	row pgx.Row
}

func (f *fakeConfirmQuerier) Exec(context.Context, string, ...any) (pgconn.CommandTag, error) {
	panic("Confirm must not call Exec")
}

func (f *fakeConfirmQuerier) Query(context.Context, string, ...any) (pgx.Rows, error) {
	panic("Confirm must not call Query")
}

func (f *fakeConfirmQuerier) QueryRow(context.Context, string, ...any) pgx.Row {
	return f.row
}

// TestPostgresLeaseStore_ConfirmCAS covers Confirm's two SQL outcomes at the
// querier level: a returned row (still ours, live) confirms; ErrNoRows (expired,
// held by a peer, or already gone) and any other query error both report false —
// Confirm never hard-errors, mirroring TryAcquire/Release's absorption contract.
func TestPostgresLeaseStore_ConfirmCAS(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		row  pgx.Row
		want bool
	}{
		{"row returned: still held by us and live", &fakeLeaseRow{holderID: "h1"}, true},
		{"no rows: expired or held by a peer", &fakeLeaseRow{err: pgx.ErrNoRows}, false},
		{"query error: transient failure absorbed as not confirmed", &fakeLeaseRow{err: errors.New("connection refused")}, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			store := newPGLeaseStore(&fakeConfirmQuerier{row: tc.row})
			got := store.Confirm(context.Background(), LeaseHandle{ETag: "h1"}, 4*time.Minute)
			if got != tc.want {
				t.Errorf("Confirm: got %v, want %v", got, tc.want)
			}
		})
	}
}

// TestPostgresLeaseStore_TryAcquireHeldWhenNoRow confirms that when the atomic
// INSERT/UPDATE does not return a row (live lease held by a peer), TryAcquire
// reports Held=true and does not set Acquired.
func TestPostgresLeaseStore_TryAcquireHeldWhenNoRow(t *testing.T) {
	t.Parallel()
	store := newPGLeaseStore(&fakeLeaseQuerier{acquireRowErr: pgx.ErrNoRows})
	res, err := store.TryAcquire(context.Background(), 4*time.Minute)
	if err != nil {
		t.Fatalf("TryAcquire: %v", err)
	}
	if res.Acquired {
		t.Error("expected not acquired (live lease)")
	}
	if !res.Held {
		t.Error("expected Held=true")
	}
}

// TestPostgresLeaseStore_TryAcquireTransientOnQueryError confirms that a DB
// error surfaces as TransientErr (not as a hard error return).
func TestPostgresLeaseStore_TryAcquireTransientOnQueryError(t *testing.T) {
	t.Parallel()
	boom := errors.New("connection refused")
	store := newPGLeaseStore(&fakeLeaseQuerier{acquireRowErr: boom})
	res, err := store.TryAcquire(context.Background(), 4*time.Minute)
	if err != nil {
		t.Fatalf("TryAcquire must not return a hard error for a transient: %v", err)
	}
	if res.Acquired || res.Held {
		t.Errorf("transient should be neither acquired nor held: %+v", res)
	}
	if res.TransientErr == nil {
		t.Error("expected TransientErr to be set")
	}
}

// TestPostgresLeaseStore_TryAcquireWonWhenRowReturned confirms that when the
// atomic query returns a holder_id row, TryAcquire reports Acquired=true and
// populates Handle.ETag with the holder id.
func TestPostgresLeaseStore_TryAcquireWonWhenRowReturned(t *testing.T) {
	t.Parallel()
	store := newPGLeaseStore(&fakeLeaseQuerier{}) // nil acquireRowErr → default row returned
	res, err := store.TryAcquire(context.Background(), 4*time.Minute)
	if err != nil {
		t.Fatalf("TryAcquire: %v", err)
	}
	if !res.Acquired {
		t.Fatalf("expected acquired, got %+v", res)
	}
	if res.Handle.ETag == "" {
		t.Error("expected non-empty Handle.ETag")
	}
}

// TestPostgresLeaseStore_ReleaseTransientOnExecError confirms that an Exec
// failure surfaces as LeaseTransientError (not a hard error return from Release).
func TestPostgresLeaseStore_ReleaseTransientOnExecError(t *testing.T) {
	t.Parallel()
	boom := errors.New("deadlock")
	store := newPGLeaseStore(&fakeLeaseQuerier{releaseExecErr: boom})
	outcome := store.Release(context.Background(), LeaseHandle{ETag: "h"})
	if outcome != LeaseTransientError {
		t.Errorf("Release: got %v, want LeaseTransientError", outcome)
	}
}
