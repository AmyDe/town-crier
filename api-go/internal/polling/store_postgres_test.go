package polling

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// fakeQuerier is a hand-written test double for the polling store's querier
// interface (Exec / Query / QueryRow). It covers the error-path branches that
// do not require a real database, keeping the unit-test layer Docker-free.
type fakeQuerier struct {
	execTag pgconn.CommandTag
	execErr error
	rowErr  error // returned from QueryRow().Scan(...)
}

// fakePollRow implements pgx.Row, returning a pre-configured error from Scan.
type fakePollRow struct{ err error }

func (r *fakePollRow) Scan(_ ...any) error { return r.err }

// fakePollRows implements pgx.Rows, always reporting no rows (used for the
// empty-result Query path).
type fakePollRows struct{}

func (r *fakePollRows) Next() bool                                   { return false }
func (r *fakePollRows) Scan(_ ...any) error                          { return nil }
func (r *fakePollRows) Values() ([]any, error)                       { return nil, nil }
func (r *fakePollRows) Close()                                       {}
func (r *fakePollRows) Err() error                                   { return nil }
func (r *fakePollRows) RawValues() [][]byte                          { return nil }
func (r *fakePollRows) FieldDescriptions() []pgconn.FieldDescription { return nil }
func (r *fakePollRows) CommandTag() pgconn.CommandTag                { return pgconn.CommandTag{} }
func (r *fakePollRows) Conn() *pgx.Conn                              { return nil }

func (f *fakeQuerier) Exec(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
	return f.execTag, f.execErr
}

func (f *fakeQuerier) Query(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
	return &fakePollRows{}, nil
}

func (f *fakeQuerier) QueryRow(_ context.Context, _ string, _ ...any) pgx.Row {
	return &fakePollRow{err: f.rowErr}
}

// Compile-time check: the Postgres store satisfies PollStateAccess.
var _ PollStateAccess = (*PostgresPollStateStore)(nil)

// TestPostgresPollStateStore_GetMissingReturnsNotFound confirms that a miss
// (pgx.ErrNoRows from the point-read query) surfaces as (zero, false, nil).
func TestPostgresPollStateStore_GetMissingReturnsNotFound(t *testing.T) {
	t.Parallel()
	store := NewPostgresPollStateStore(&fakeQuerier{rowErr: pgx.ErrNoRows})
	_, ok, err := store.Get(context.Background(), 42)
	if err != nil {
		t.Fatalf("Get: expected nil error on miss, got %v", err)
	}
	if ok {
		t.Error("Get: expected ok=false for missing state")
	}
}

// TestPostgresPollStateStore_GetSurfacesQueryError confirms that a non-ErrNoRows
// read failure is returned as a wrapped error.
func TestPostgresPollStateStore_GetSurfacesQueryError(t *testing.T) {
	t.Parallel()
	boom := errors.New("connection refused")
	store := NewPostgresPollStateStore(&fakeQuerier{rowErr: boom})
	_, _, err := store.Get(context.Background(), 42)
	if !errors.Is(err, boom) {
		t.Fatalf("Get: expected wrapped %v, got %v", boom, err)
	}
}

// TestPostgresPollStateStore_SaveSurfacesExecError confirms that an exec failure
// is returned as a wrapped error.
func TestPostgresPollStateStore_SaveSurfacesExecError(t *testing.T) {
	t.Parallel()
	boom := errors.New("deadlock")
	store := NewPostgresPollStateStore(&fakeQuerier{execErr: boom})
	err := store.Save(context.Background(), 1,
		time.Now().UTC(), time.Now().UTC(), nil)
	if !errors.Is(err, boom) {
		t.Fatalf("Save: expected wrapped %v, got %v", boom, err)
	}
}

// TestPostgresPollStateStore_GetLRPEmptyInputShortCircuits confirms that an
// empty candidate slice returns immediately with zero results and no DB call.
func TestPostgresPollStateStore_GetLRPEmptyInputShortCircuits(t *testing.T) {
	t.Parallel()
	// fakeQuerier will panic if Query is called; it must never be.
	store := NewPostgresPollStateStore(&fakeQuerier{})
	res, err := store.GetLeastRecentlyPolled(context.Background(), nil)
	if err != nil {
		t.Fatalf("GetLeastRecentlyPolled: %v", err)
	}
	if len(res.AuthorityIDs) != 0 || res.NeverPolledCount != 0 {
		t.Errorf("expected empty result for nil input, got %+v", res)
	}
}
