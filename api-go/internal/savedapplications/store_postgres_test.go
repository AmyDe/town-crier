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
	execErr   error
	queryErr  error
	rowErr    error
	queryRows pgx.Rows // returned from Query() when non-nil (else emptySavedRows)
	countRow  pgx.Row  // returned from QueryRow() when non-nil (else errSavedRow)
	queries   int      // number of Query calls issued
}

func (f *fakeSavedQuerier) Exec(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, f.execErr
}

func (f *fakeSavedQuerier) Query(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
	f.queries++
	if f.queryErr != nil {
		return nil, f.queryErr
	}
	if f.queryRows != nil {
		return f.queryRows, nil
	}
	return &emptySavedRows{}, nil
}

func (f *fakeSavedQuerier) QueryRow(_ context.Context, _ string, _ ...any) pgx.Row {
	if f.countRow != nil {
		return f.countRow
	}
	return &errSavedRow{err: f.rowErr}
}

// countSavedRows is a pgx.Rows over pre-baked (user_id, count) tuples for the
// batched CountsByUsers query.
type countSavedRows struct {
	rows [][2]any
	idx  int
}

func (r *countSavedRows) Next() bool { return r.idx < len(r.rows) }
func (r *countSavedRows) Scan(dest ...any) error {
	*dest[0].(*string) = r.rows[r.idx][0].(string)
	*dest[1].(*int) = r.rows[r.idx][1].(int)
	r.idx++
	return nil
}
func (r *countSavedRows) Close()                                       {}
func (r *countSavedRows) Err() error                                   { return nil }
func (r *countSavedRows) CommandTag() pgconn.CommandTag                { return pgconn.CommandTag{} }
func (r *countSavedRows) FieldDescriptions() []pgconn.FieldDescription { return nil }
func (r *countSavedRows) Values() ([]any, error)                       { return nil, nil }
func (r *countSavedRows) RawValues() [][]byte                          { return nil }
func (r *countSavedRows) Conn() *pgx.Conn                              { return nil }

// intSavedRow is a pgx.Row that scans a single int (the Count total) or a
// pre-configured error.
type intSavedRow struct {
	n   int
	err error
}

func (r *intSavedRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	*dest[0].(*int) = r.n
	return nil
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

// --- CountsByUsers ---

// TestSavedPostgresStore_CountsByUsers_Empty short-circuits an empty user set
// with no query and an empty, non-nil map.
func TestSavedPostgresStore_CountsByUsers_Empty(t *testing.T) {
	t.Parallel()
	fq := &fakeSavedQuerier{}
	store := NewPostgresStore(fq)

	got, err := store.CountsByUsers(context.Background(), nil)
	if err != nil {
		t.Fatalf("CountsByUsers: %v", err)
	}
	if got == nil || len(got) != 0 {
		t.Errorf("empty users: got %v, want empty non-nil map", got)
	}
	if fq.queries != 0 {
		t.Errorf("empty users issued %d queries, want 0", fq.queries)
	}
}

// TestSavedPostgresStore_CountsByUsers_PropagatesQueryError wraps a Query error.
func TestSavedPostgresStore_CountsByUsers_PropagatesQueryError(t *testing.T) {
	t.Parallel()
	boom := errors.New("counts boom")
	fq := &fakeSavedQuerier{queryErr: boom}
	store := NewPostgresStore(fq)

	if _, err := store.CountsByUsers(context.Background(), []string{"auth0|u1"}); !errors.Is(err, boom) {
		t.Fatalf("got %v, want wrapped %v", err, boom)
	}
}

// TestSavedPostgresStore_CountsByUsers_MapsPerUser maps one count per user; a
// user absent from the grouped result is omitted (defaults to 0 at the call site).
func TestSavedPostgresStore_CountsByUsers_MapsPerUser(t *testing.T) {
	t.Parallel()
	fq := &fakeSavedQuerier{queryRows: &countSavedRows{rows: [][2]any{
		{"auth0|u1", 5},
		{"auth0|u2", 1},
	}}}
	store := NewPostgresStore(fq)

	got, err := store.CountsByUsers(context.Background(), []string{"auth0|u1", "auth0|u2", "auth0|u3"})
	if err != nil {
		t.Fatalf("CountsByUsers: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("map size: got %d, want 2 (absent users omitted)", len(got))
	}
	if got["auth0|u1"] != 5 || got["auth0|u2"] != 1 {
		t.Errorf("counts: got %+v, want {u1:5 u2:1}", got)
	}
	if _, ok := got["auth0|u3"]; ok {
		t.Error("auth0|u3 must be absent (defaults to zero at the call site)")
	}
}

// --- Count ---

// TestSavedPostgresStore_Count_ReturnsTotal returns the scalar count(*).
func TestSavedPostgresStore_Count_ReturnsTotal(t *testing.T) {
	t.Parallel()
	fq := &fakeSavedQuerier{countRow: &intSavedRow{n: 42}}
	store := NewPostgresStore(fq)

	got, err := store.Count(context.Background())
	if err != nil {
		t.Fatalf("Count: %v", err)
	}
	if got != 42 {
		t.Errorf("Count = %d, want 42", got)
	}
}

// TestSavedPostgresStore_Count_PropagatesError wraps a scan error.
func TestSavedPostgresStore_Count_PropagatesError(t *testing.T) {
	t.Parallel()
	boom := errors.New("count boom")
	fq := &fakeSavedQuerier{countRow: &intSavedRow{err: boom}}
	store := NewPostgresStore(fq)

	if _, err := store.Count(context.Background()); !errors.Is(err, boom) {
		t.Fatalf("got %v, want wrapped %v", err, boom)
	}
}
