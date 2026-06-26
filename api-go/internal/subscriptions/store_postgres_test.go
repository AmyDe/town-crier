package subscriptions

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// fakeBoolRow implements pgx.Row, scanning a single bool result. Used by
// fakeQuerier to simulate the SELECT EXISTS query in IsProcessed.
type fakeBoolRow struct {
	val bool
	err error
}

func (r *fakeBoolRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	*dest[0].(*bool) = r.val //nolint:forcetypeassert // controlled in tests
	return nil
}

// fakeQuerier implements querier without a real database.
// rowResult/rowErr feed QueryRow; execErr/execCalls feed Exec.
type fakeQuerier struct {
	rowResult bool
	rowErr    error
	execErr   error
	execCalls []execCall
}

type execCall struct {
	sql  string
	args []any
}

func (f *fakeQuerier) Exec(_ context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	f.execCalls = append(f.execCalls, execCall{sql: sql, args: args})
	return pgconn.CommandTag{}, f.execErr
}

func (f *fakeQuerier) Query(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
	return nil, errors.New("Query not used by PostgresNotificationStore")
}

func (f *fakeQuerier) QueryRow(_ context.Context, _ string, _ ...any) pgx.Row {
	return &fakeBoolRow{val: f.rowResult, err: f.rowErr}
}

// TestPostgresNotificationStore_IsProcessed_Miss returns false when no row exists.
func TestPostgresNotificationStore_IsProcessed_Miss(t *testing.T) {
	t.Parallel()
	q := &fakeQuerier{rowResult: false}
	store := NewPostgresNotificationStore(q, time.Now)

	got, err := store.IsProcessed(context.Background(), "uuid-1")
	if err != nil {
		t.Fatalf("IsProcessed: %v", err)
	}
	if got {
		t.Error("want not processed for absent uuid, got true")
	}
}

// TestPostgresNotificationStore_IsProcessed_Hit returns true when the row exists.
func TestPostgresNotificationStore_IsProcessed_Hit(t *testing.T) {
	t.Parallel()
	q := &fakeQuerier{rowResult: true}
	store := NewPostgresNotificationStore(q, time.Now)

	got, err := store.IsProcessed(context.Background(), "uuid-2")
	if err != nil {
		t.Fatalf("IsProcessed: %v", err)
	}
	if !got {
		t.Error("want processed for present uuid, got false")
	}
}

// TestPostgresNotificationStore_IsProcessed_Error wraps database errors.
func TestPostgresNotificationStore_IsProcessed_Error(t *testing.T) {
	t.Parallel()
	sentinel := errors.New("db scan error")
	q := &fakeQuerier{rowErr: sentinel}
	store := NewPostgresNotificationStore(q, time.Now)

	_, err := store.IsProcessed(context.Background(), "uuid-3")
	if !errors.Is(err, sentinel) {
		t.Fatalf("got err %v, want wrapped sentinel", err)
	}
}

// TestPostgresNotificationStore_MarkProcessed calls Exec once with the uuid
// and the injected timestamp.
func TestPostgresNotificationStore_MarkProcessed(t *testing.T) {
	t.Parallel()
	fixed := time.Date(2026, 6, 26, 12, 0, 0, 0, time.UTC)
	q := &fakeQuerier{}
	store := NewPostgresNotificationStore(q, func() time.Time { return fixed })

	if err := store.MarkProcessed(context.Background(), "uuid-4"); err != nil {
		t.Fatalf("MarkProcessed: %v", err)
	}
	if len(q.execCalls) != 1 {
		t.Fatalf("expected 1 exec call, got %d", len(q.execCalls))
	}
	call := q.execCalls[0]
	if len(call.args) < 2 {
		t.Fatalf("expected at least 2 args, got %d", len(call.args))
	}
	if call.args[0] != "uuid-4" {
		t.Errorf("first arg = %v, want uuid-4", call.args[0])
	}
	if call.args[1] != fixed {
		t.Errorf("second arg = %v, want %v", call.args[1], fixed)
	}
}

// TestPostgresNotificationStore_MarkProcessed_CallsTwice proves MarkProcessed
// can be called twice without error at the code level. The ON CONFLICT DO UPDATE
// idempotency against a real unique constraint is exercised in the integration
// test (TestPostgresNotificationStore_MarkProcessed_Idempotent).
func TestPostgresNotificationStore_MarkProcessed_CallsTwice(t *testing.T) {
	t.Parallel()
	q := &fakeQuerier{}
	store := NewPostgresNotificationStore(q, time.Now)

	if err := store.MarkProcessed(context.Background(), "uuid-6"); err != nil {
		t.Fatalf("first MarkProcessed: %v", err)
	}
	if err := store.MarkProcessed(context.Background(), "uuid-6"); err != nil {
		t.Fatalf("second MarkProcessed (idempotent): %v", err)
	}
	if len(q.execCalls) != 2 {
		t.Errorf("expected 2 exec calls, got %d", len(q.execCalls))
	}
}

// TestPostgresNotificationStore_MarkProcessed_Error wraps database errors.
func TestPostgresNotificationStore_MarkProcessed_Error(t *testing.T) {
	t.Parallel()
	sentinel := errors.New("db exec error")
	q := &fakeQuerier{execErr: sentinel}
	store := NewPostgresNotificationStore(q, time.Now)

	if err := store.MarkProcessed(context.Background(), "uuid-5"); !errors.Is(err, sentinel) {
		t.Fatalf("got err %v, want wrapped sentinel", err)
	}
}

// TestPostgresNotificationStore_UpsertProcessed calls Exec with explicit processedAt.
func TestPostgresNotificationStore_UpsertProcessed(t *testing.T) {
	t.Parallel()
	at := time.Date(2026, 1, 15, 9, 0, 0, 0, time.UTC)
	q := &fakeQuerier{}
	store := NewPostgresNotificationStore(q, time.Now)

	if err := store.UpsertProcessed(context.Background(), "uuid-7", at); err != nil {
		t.Fatalf("UpsertProcessed: %v", err)
	}
	if len(q.execCalls) != 1 {
		t.Fatalf("expected 1 exec call, got %d", len(q.execCalls))
	}
	call := q.execCalls[0]
	if call.args[0] != "uuid-7" {
		t.Errorf("first arg = %v, want uuid-7", call.args[0])
	}
	if call.args[1] != at {
		t.Errorf("second arg = %v, want %v", call.args[1], at)
	}
}
