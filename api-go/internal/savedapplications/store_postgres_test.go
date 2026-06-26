package savedapplications

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/AmyDe/town-crier/api-go/internal/applications"
)

// fakeSavedQuerier is a hand-written fake of the querier interface used by
// unit tests of PostgresStore (savedapplications). It stores canned responses for
// each method; tests set whichever fields they exercise.
type fakeSavedQuerier struct {
	execErr  error
	queryErr error
	rowErr   error
}

func (f *fakeSavedQuerier) Exec(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, f.execErr
}

func (f *fakeSavedQuerier) Query(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
	if f.queryErr != nil {
		return nil, f.queryErr
	}
	return &emptySavedRows{}, nil
}

func (f *fakeSavedQuerier) QueryRow(_ context.Context, _ string, _ ...any) pgx.Row {
	return &errSavedRow{err: f.rowErr}
}

// emptySavedRows is a pgx.Rows with no data and no error.
type emptySavedRows struct{}

func (r *emptySavedRows) Close()                                       {}
func (r *emptySavedRows) Err() error                                   { return nil }
func (r *emptySavedRows) CommandTag() pgconn.CommandTag                { return pgconn.CommandTag{} }
func (r *emptySavedRows) FieldDescriptions() []pgconn.FieldDescription { return nil }
func (r *emptySavedRows) Next() bool                                   { return false }
func (r *emptySavedRows) Scan(_ ...any) error                          { return errors.New("no rows") }
func (r *emptySavedRows) Values() ([]any, error)                       { return nil, nil }
func (r *emptySavedRows) RawValues() [][]byte                          { return nil }
func (r *emptySavedRows) Conn() *pgx.Conn                              { return nil }

// errSavedRow is a pgx.Row whose Scan always returns the stored error.
type errSavedRow struct{ err error }

func (r *errSavedRow) Scan(_ ...any) error { return r.err }

// newTestApp builds a minimal PlanningApplication for fixture use.
func newTestApp() applications.PlanningApplication {
	return applications.PlanningApplication{
		Name:          "24/00001/FUL",
		UID:           "uid-1",
		AreaName:      "Testshire",
		AreaID:        100,
		Address:       "1 Test Street",
		Description:   "test application",
		LastDifferent: time.Date(2026, 6, 26, 12, 0, 0, 0, time.UTC),
	}
}

// TestSavedPostgresStore_Save_PropagatesExecError surfaces an Exec failure.
func TestSavedPostgresStore_Save_PropagatesExecError(t *testing.T) {
	t.Parallel()

	sentinel := errors.New("db down")
	fq := &fakeSavedQuerier{execErr: sentinel}
	store := NewPostgresStore(fq)

	app := newTestApp()
	sa := NewSavedApplication("auth0|u1", app, time.Now())
	if err := store.Save(context.Background(), sa); !errors.Is(err, sentinel) {
		t.Fatalf("Save error: got %v, want wrapped %v", err, sentinel)
	}
}

// TestSavedPostgresStore_Exists_FalseOnNoRows returns false when QueryRow.Scan
// returns pgx.ErrNoRows (the row does not exist).
func TestSavedPostgresStore_Exists_FalseOnNoRows(t *testing.T) {
	t.Parallel()

	fq := &fakeSavedQuerier{rowErr: pgx.ErrNoRows}
	store := NewPostgresStore(fq)

	got, err := store.Exists(context.Background(), "auth0|u1", "100/24/00001/FUL")
	if err != nil {
		t.Fatalf("Exists missing: got err %v, want nil", err)
	}
	if got {
		t.Error("Exists missing: got true, want false")
	}
}

// TestSavedPostgresStore_Exists_DBError surfaces a non-ErrNoRows scan error.
func TestSavedPostgresStore_Exists_DBError(t *testing.T) {
	t.Parallel()

	sentinel := errors.New("db down")
	fq := &fakeSavedQuerier{rowErr: sentinel}
	store := NewPostgresStore(fq)

	if _, err := store.Exists(context.Background(), "auth0|u1", "uid"); !errors.Is(err, sentinel) {
		t.Fatalf("Exists DB error: got %v, want wrapped %v", err, sentinel)
	}
}

// TestSavedPostgresStore_Delete_PropagatesExecError surfaces an Exec failure.
func TestSavedPostgresStore_Delete_PropagatesExecError(t *testing.T) {
	t.Parallel()

	sentinel := errors.New("db down")
	fq := &fakeSavedQuerier{execErr: sentinel}
	store := NewPostgresStore(fq)

	if err := store.Delete(context.Background(), "auth0|u1", "uid"); !errors.Is(err, sentinel) {
		t.Fatalf("Delete error: got %v, want wrapped %v", err, sentinel)
	}
}

// TestSavedPostgresStore_GetByUserID_EmptyReturnsNil returns nil (not an empty
// slice) when the user has no saved applications.
func TestSavedPostgresStore_GetByUserID_EmptyReturnsNil(t *testing.T) {
	t.Parallel()

	fq := &fakeSavedQuerier{}
	store := NewPostgresStore(fq)

	got, err := store.GetByUserID(context.Background(), "auth0|u1")
	if err != nil {
		t.Fatalf("GetByUserID empty: got err %v", err)
	}
	if got != nil {
		t.Errorf("GetByUserID empty: got %v, want nil", got)
	}
}

// TestSavedPostgresStore_GetByUserID_QueryError surfaces a Query failure.
func TestSavedPostgresStore_GetByUserID_QueryError(t *testing.T) {
	t.Parallel()

	sentinel := errors.New("db down")
	fq := &fakeSavedQuerier{queryErr: sentinel}
	store := NewPostgresStore(fq)

	if _, err := store.GetByUserID(context.Background(), "auth0|u1"); !errors.Is(err, sentinel) {
		t.Fatalf("GetByUserID query error: got %v, want wrapped %v", err, sentinel)
	}
}

// TestSavedPostgresStore_UserIDsForApplication_EmptyReturnsNil returns nil when
// no users have saved the application.
func TestSavedPostgresStore_UserIDsForApplication_EmptyReturnsNil(t *testing.T) {
	t.Parallel()

	fq := &fakeSavedQuerier{}
	store := NewPostgresStore(fq)

	got, err := store.UserIDsForApplication(context.Background(), "100/24/00001/FUL", 100)
	if err != nil {
		t.Fatalf("UserIDsForApplication empty: got err %v", err)
	}
	if got != nil {
		t.Errorf("UserIDsForApplication empty: got %v, want nil", got)
	}
}

// TestSavedPostgresStore_UserIDsForApplication_QueryError surfaces a Query failure.
func TestSavedPostgresStore_UserIDsForApplication_QueryError(t *testing.T) {
	t.Parallel()

	sentinel := errors.New("db down")
	fq := &fakeSavedQuerier{queryErr: sentinel}
	store := NewPostgresStore(fq)

	if _, err := store.UserIDsForApplication(context.Background(), "uid", 100); !errors.Is(err, sentinel) {
		t.Fatalf("UserIDsForApplication query error: got %v, want wrapped %v", err, sentinel)
	}
}

// TestSavedPostgresStore_DeleteAllByUserID_PropagatesExecError surfaces an Exec failure.
func TestSavedPostgresStore_DeleteAllByUserID_PropagatesExecError(t *testing.T) {
	t.Parallel()

	sentinel := errors.New("db down")
	fq := &fakeSavedQuerier{execErr: sentinel}
	store := NewPostgresStore(fq)

	if err := store.DeleteAllByUserID(context.Background(), "auth0|u1"); !errors.Is(err, sentinel) {
		t.Fatalf("DeleteAllByUserID error: got %v, want wrapped %v", err, sentinel)
	}
}
