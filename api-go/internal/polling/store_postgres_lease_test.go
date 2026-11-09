package polling

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type fakeLeaseQuerier struct {
	acquireRowErr error

	releaseExecTag pgconn.CommandTag
	releaseExecErr error

	existsCount int
	existsErr   error
}

// fakeLeaseRow is the TryAcquire RETURNING row: pgx.ErrNoRows means held, any
// other error is transient.
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

func (f *fakeLeaseQuerier) Exec(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
	return f.releaseExecTag, f.releaseExecErr
}

func (f *fakeLeaseQuerier) Query(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
	return &fakePollRows{}, nil
}

func (f *fakeLeaseQuerier) QueryRow(_ context.Context, _ string, _ ...any) pgx.Row {
	if f.releaseExecTag.RowsAffected() > 0 {
		return &fakeExistsRow{count: 0}
	}
	if f.releaseExecErr == nil && f.acquireRowErr == nil {
		return &fakeLeaseRow{holderID: "test-holder"}
	}
	if f.acquireRowErr != nil {
		return &fakeLeaseRow{err: f.acquireRowErr}
	}
	return &fakeExistsRow{count: f.existsCount, err: f.existsErr}
}

func newPGLeaseStore(q querier) *PostgresLeaseStore {
	clock := func() time.Time { return time.Date(2026, 6, 14, 12, 0, 0, 0, time.UTC) }
	return NewPostgresLeaseStore(q, clock)
}

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

func TestPostgresLeaseStore_ReleaseTransientOnExecError(t *testing.T) {
	t.Parallel()
	boom := errors.New("deadlock")
	store := newPGLeaseStore(&fakeLeaseQuerier{releaseExecErr: boom})
	outcome := store.Release(context.Background(), LeaseHandle{ETag: "h"})
	if outcome != LeaseTransientError {
		t.Errorf("Release: got %v, want LeaseTransientError", outcome)
	}
}

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
