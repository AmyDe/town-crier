package profiles

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// fakeExecQuerier is a hand-written test double for the querier interface that
// supports only Exec — the three Exec-based methods (Save, Delete,
// UpdateZoneCountWithCAS) are the ones whose error paths and CAS logic can be
// exercised without a real database. Query and QueryRow panic when called, so
// any unintended use is caught immediately rather than silently no-oping.
type fakeExecQuerier struct {
	tag pgconn.CommandTag
	err error
}

func (f *fakeExecQuerier) Exec(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
	return f.tag, f.err
}

func (f *fakeExecQuerier) Query(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
	panic("fakeExecQuerier.Query not expected in this test")
}

func (f *fakeExecQuerier) QueryRow(_ context.Context, _ string, _ ...any) pgx.Row {
	panic("fakeExecQuerier.QueryRow not expected in this test")
}

// newFakePGStore builds a PostgresStore backed by fakeExecQuerier.
func newFakePGStore(tag pgconn.CommandTag, err error) *PostgresStore {
	return NewPostgresStore(&fakeExecQuerier{tag: tag, err: err})
}

// testProfile builds a minimal valid UserProfile for use in store tests.
// The userID parameter is intentionally parameterised for readability even
// though unit tests call it with the same argument — it is also used by the
// integration test suite in the same package.
//
//nolint:unparam
func testProfile(userID string) *UserProfile {
	p, _ := NewProfile(userID, "test@example.com", time.Date(2026, 6, 26, 12, 0, 0, 0, time.UTC))
	return p
}

// TestPostgresStore_Save_WrapsDatabaseError verifies that a database error from
// Exec is wrapped and returned — never swallowed.
func TestPostgresStore_Save_WrapsDatabaseError(t *testing.T) {
	t.Parallel()
	sentinel := errors.New("db exploded")
	store := newFakePGStore(pgconn.CommandTag{}, sentinel)

	err := store.Save(context.Background(), testProfile("auth0|u1"))
	if !errors.Is(err, sentinel) {
		t.Fatalf("Save: got %v, want wrapped %v", err, sentinel)
	}
}

// TestPostgresStore_Delete_MissReturnsErrNotFound verifies that zero rows
// affected on the DELETE statement surfaces as ErrNotFound (not a wrapped
// database error), matching the Cosmos store's contract.
func TestPostgresStore_Delete_MissReturnsErrNotFound(t *testing.T) {
	t.Parallel()
	// CommandTag "DELETE 0" → RowsAffected() == 0
	store := newFakePGStore(pgconn.NewCommandTag("DELETE 0"), nil)

	err := store.Delete(context.Background(), "auth0|missing")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("Delete miss: got %v, want ErrNotFound", err)
	}
}

// TestPostgresStore_Delete_SuccessReturnsNil verifies that exactly one affected
// row from the DELETE yields nil.
func TestPostgresStore_Delete_SuccessReturnsNil(t *testing.T) {
	t.Parallel()
	store := newFakePGStore(pgconn.NewCommandTag("DELETE 1"), nil)

	if err := store.Delete(context.Background(), "auth0|u1"); err != nil {
		t.Fatalf("Delete success: got %v, want nil", err)
	}
}

// TestPostgresStore_Delete_WrapsDatabaseError verifies that a database error is
// returned even when the rows-affected count is irrelevant.
func TestPostgresStore_Delete_WrapsDatabaseError(t *testing.T) {
	t.Parallel()
	sentinel := errors.New("connection refused")
	store := newFakePGStore(pgconn.CommandTag{}, sentinel)

	err := store.Delete(context.Background(), "auth0|u1")
	if !errors.Is(err, sentinel) {
		t.Fatalf("Delete error: got %v, want wrapped %v", err, sentinel)
	}
}

// TestPostgresStore_UpdateZoneCountWithCAS_InvalidEtag verifies that a
// non-numeric etag (e.g. a raw Cosmos _etag string) is rejected with a
// descriptive error before any SQL is issued.
func TestPostgresStore_UpdateZoneCountWithCAS_InvalidEtag(t *testing.T) {
	t.Parallel()
	// Any tag/err — should not be reached since etag parsing fails first.
	store := newFakePGStore(pgconn.NewCommandTag("UPDATE 1"), nil)

	err := store.UpdateZoneCountWithCAS(context.Background(), "auth0|u1", testProfile("auth0|u1"), "not-a-number")
	if err == nil {
		t.Fatal("expected error for invalid etag, got nil")
	}
}

// TestPostgresStore_UpdateZoneCountWithCAS_ConflictReturnsCASError verifies
// that zero rows affected from the conditional UPDATE returns the exact
// platform.ErrCASPreconditionFailed sentinel (wrapped), matching the contract
// the watchzones quota CAS handler branches on.
func TestPostgresStore_UpdateZoneCountWithCAS_ConflictReturnsCASError(t *testing.T) {
	t.Parallel()
	// CommandTag "UPDATE 0" → RowsAffected() == 0 → precondition failed
	store := newFakePGStore(pgconn.NewCommandTag("UPDATE 0"), nil)

	err := store.UpdateZoneCountWithCAS(context.Background(), "auth0|u1", testProfile("auth0|u1"), "3")
	if err == nil {
		t.Fatal("expected CAS error, got nil")
	}
	// Must be detectable via errors.Is so the handler can branch.
	if !errors.Is(err, errCASPreconditionFailed) {
		t.Fatalf("UpdateZoneCountWithCAS conflict: got %v, want to wrap errCASPreconditionFailed", err)
	}
}

// TestPostgresStore_UpdateZoneCountWithCAS_SuccessReturnsNil verifies that one
// row affected from the conditional UPDATE yields nil.
func TestPostgresStore_UpdateZoneCountWithCAS_SuccessReturnsNil(t *testing.T) {
	t.Parallel()
	store := newFakePGStore(pgconn.NewCommandTag("UPDATE 1"), nil)

	if err := store.UpdateZoneCountWithCAS(context.Background(), "auth0|u1", testProfile("auth0|u1"), "2"); err != nil {
		t.Fatalf("UpdateZoneCountWithCAS success: got %v, want nil", err)
	}
}

// TestPostgresStore_UpdateZoneCountWithCAS_WrapsDatabaseError verifies that a
// database error from Exec is wrapped and returned.
func TestPostgresStore_UpdateZoneCountWithCAS_WrapsDatabaseError(t *testing.T) {
	t.Parallel()
	sentinel := errors.New("db timeout")
	store := newFakePGStore(pgconn.CommandTag{}, sentinel)

	err := store.UpdateZoneCountWithCAS(context.Background(), "auth0|u1", testProfile("auth0|u1"), "0")
	if !errors.Is(err, sentinel) {
		t.Fatalf("UpdateZoneCountWithCAS db error: got %v, want wrapped %v", err, sentinel)
	}
}
