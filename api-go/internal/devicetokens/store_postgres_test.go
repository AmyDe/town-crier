package devicetokens

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// fakeQuerier is a hand-written fake implementing the querier interface for
// error-path unit tests of PostgresStore. It stores canned responses for each
// method; tests set whichever fields they exercise.
type fakeQuerier struct {
	execTag  pgconn.CommandTag
	execErr  error
	queryErr error
	rowErr   error
}

func (f *fakeQuerier) Exec(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
	return f.execTag, f.execErr
}

func (f *fakeQuerier) Query(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
	if f.queryErr != nil {
		return nil, f.queryErr
	}
	return &emptyDeviceRows{}, nil
}

func (f *fakeQuerier) QueryRow(_ context.Context, _ string, _ ...any) pgx.Row {
	return &errDeviceRow{err: f.rowErr}
}

// emptyDeviceRows is a pgx.Rows that reports no rows and no error. It satisfies
// the full pgx.Rows interface (including the Conn method added in pgx v5).
type emptyDeviceRows struct{}

func (r *emptyDeviceRows) Close()                                       {}
func (r *emptyDeviceRows) Err() error                                   { return nil }
func (r *emptyDeviceRows) CommandTag() pgconn.CommandTag                { return pgconn.CommandTag{} }
func (r *emptyDeviceRows) FieldDescriptions() []pgconn.FieldDescription { return nil }
func (r *emptyDeviceRows) Next() bool                                   { return false }
func (r *emptyDeviceRows) Scan(_ ...any) error                          { return errors.New("no rows") }
func (r *emptyDeviceRows) Values() ([]any, error)                       { return nil, nil }
func (r *emptyDeviceRows) RawValues() [][]byte                          { return nil }
func (r *emptyDeviceRows) Conn() *pgx.Conn                              { return nil }

// errDeviceRow is a pgx.Row whose Scan always returns the stored error (or nil).
type errDeviceRow struct{ err error }

func (r *errDeviceRow) Scan(_ ...any) error { return r.err }

// TestPostgresStore_GetByToken_Missing returns (nil, nil) when the row is absent —
// the "not registered yet" signal the PUT handler branches on.
func TestPostgresStore_GetByToken_Missing(t *testing.T) {
	t.Parallel()

	fq := &fakeQuerier{rowErr: pgx.ErrNoRows}
	store := NewPostgresStore(fq)

	got, err := store.GetByToken(context.Background(), "auth0|u1", "tok-missing")
	if err != nil {
		t.Fatalf("GetByToken missing: got err %v, want nil", err)
	}
	if got != nil {
		t.Errorf("GetByToken missing: got %+v, want nil", got)
	}
}

// TestPostgresStore_GetByToken_DBError surfaces a non-ErrNoRows error from the
// database.
func TestPostgresStore_GetByToken_DBError(t *testing.T) {
	t.Parallel()

	sentinel := errors.New("db down")
	fq := &fakeQuerier{rowErr: sentinel}
	store := NewPostgresStore(fq)

	_, err := store.GetByToken(context.Background(), "auth0|u1", "tok")
	if !errors.Is(err, sentinel) {
		t.Fatalf("GetByToken DB error: got %v, want wrapped %v", err, sentinel)
	}
}

// TestPostgresStore_Save_PropagatesExecError surfaces an Exec failure.
func TestPostgresStore_Save_PropagatesExecError(t *testing.T) {
	t.Parallel()

	sentinel := errors.New("db down")
	fq := &fakeQuerier{execErr: sentinel}
	store := NewPostgresStore(fq)

	reg, _ := NewRegistration("auth0|u1", "tok", PlatformIos, time.Now())
	if err := store.Save(context.Background(), reg); !errors.Is(err, sentinel) {
		t.Fatalf("Save error: got %v, want wrapped %v", err, sentinel)
	}
}

// TestPostgresStore_Delete_PropagatesExecError surfaces an Exec failure.
func TestPostgresStore_Delete_PropagatesExecError(t *testing.T) {
	t.Parallel()

	sentinel := errors.New("db down")
	fq := &fakeQuerier{execErr: sentinel}
	store := NewPostgresStore(fq)

	if err := store.Delete(context.Background(), "auth0|u1", "tok"); !errors.Is(err, sentinel) {
		t.Fatalf("Delete error: got %v, want wrapped %v", err, sentinel)
	}
}

// TestPostgresStore_ListByUser_EmptyReturnsNil returns nil (not an empty slice)
// when the user has no registrations — consistent with the zero-value slice and
// callers that range over the result.
func TestPostgresStore_ListByUser_EmptyReturnsNil(t *testing.T) {
	t.Parallel()

	fq := &fakeQuerier{}
	store := NewPostgresStore(fq)

	got, err := store.ListByUser(context.Background(), "auth0|u1")
	if err != nil {
		t.Fatalf("ListByUser empty: got err %v", err)
	}
	if got != nil {
		t.Errorf("ListByUser empty: got %v, want nil", got)
	}
}

// TestPostgresStore_ListByUser_QueryError surfaces a Query failure.
func TestPostgresStore_ListByUser_QueryError(t *testing.T) {
	t.Parallel()

	sentinel := errors.New("db down")
	fq := &fakeQuerier{queryErr: sentinel}
	store := NewPostgresStore(fq)

	if _, err := store.ListByUser(context.Background(), "auth0|u1"); !errors.Is(err, sentinel) {
		t.Fatalf("ListByUser query error: got %v, want wrapped %v", err, sentinel)
	}
}

// TestPostgresStore_DeleteAllByUserID_PropagatesExecError surfaces an Exec failure.
func TestPostgresStore_DeleteAllByUserID_PropagatesExecError(t *testing.T) {
	t.Parallel()

	sentinel := errors.New("db down")
	fq := &fakeQuerier{execErr: sentinel}
	store := NewPostgresStore(fq)

	if err := store.DeleteAllByUserID(context.Background(), "auth0|u1"); !errors.Is(err, sentinel) {
		t.Fatalf("DeleteAllByUserID error: got %v, want wrapped %v", err, sentinel)
	}
}

// TestPostgresStore_PurgeOlderThan_ReturnsRowCount verifies that PurgeOlderThan
// returns the RowsAffected count from Exec.
func TestPostgresStore_PurgeOlderThan_ReturnsRowCount(t *testing.T) {
	t.Parallel()

	fq := &fakeQuerier{execTag: pgconn.NewCommandTag("DELETE 7")}
	store := NewPostgresStore(fq)

	got, err := store.PurgeOlderThan(context.Background(), time.Now())
	if err != nil {
		t.Fatalf("PurgeOlderThan: got err %v", err)
	}
	if got != 7 {
		t.Errorf("PurgeOlderThan rows affected: got %d, want 7", got)
	}
}

// TestPostgresStore_PurgeOlderThan_PropagatesExecError surfaces an Exec failure.
func TestPostgresStore_PurgeOlderThan_PropagatesExecError(t *testing.T) {
	t.Parallel()

	sentinel := errors.New("db down")
	fq := &fakeQuerier{execErr: sentinel}
	store := NewPostgresStore(fq)

	if _, err := store.PurgeOlderThan(context.Background(), time.Now()); !errors.Is(err, sentinel) {
		t.Fatalf("PurgeOlderThan error: got %v, want wrapped %v", err, sentinel)
	}
}
