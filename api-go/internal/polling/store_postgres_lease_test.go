package polling

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

var _ LeaseAccess = (*PostgresLeaseStore)(nil)

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

// fakeConfirmQuerier panics on Exec and Query because Confirm uses only QueryRow.
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
