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
	execTag   pgconn.CommandTag
	execErr   error
	queryErr  error
	rowErr    error
	queryRows pgx.Rows // returned from Query() when non-nil (else emptyDeviceRows)
	countRow  pgx.Row  // returned from QueryRow() when non-nil (else errDeviceRow)
	queries   int      // number of Query calls issued
}

func (f *fakeQuerier) Exec(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
	return f.execTag, f.execErr
}

func (f *fakeQuerier) Query(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
	f.queries++
	if f.queryErr != nil {
		return nil, f.queryErr
	}
	if f.queryRows != nil {
		return f.queryRows, nil
	}
	return &emptyDeviceRows{}, nil
}

func (f *fakeQuerier) QueryRow(_ context.Context, _ string, _ ...any) pgx.Row {
	if f.countRow != nil {
		return f.countRow
	}
	return &errDeviceRow{err: f.rowErr}
}

// countDeviceRows is a pgx.Rows over pre-baked (user_id, count) tuples for the
// batched CountsByUsers query.
type countDeviceRows struct {
	rows [][2]any
	idx  int
}

func (r *countDeviceRows) Next() bool { return r.idx < len(r.rows) }
func (r *countDeviceRows) Scan(dest ...any) error {
	*dest[0].(*string) = r.rows[r.idx][0].(string)
	*dest[1].(*int) = r.rows[r.idx][1].(int)
	r.idx++
	return nil
}
func (r *countDeviceRows) Close()                                       {}
func (r *countDeviceRows) Err() error                                   { return nil }
func (r *countDeviceRows) CommandTag() pgconn.CommandTag                { return pgconn.CommandTag{} }
func (r *countDeviceRows) FieldDescriptions() []pgconn.FieldDescription { return nil }
func (r *countDeviceRows) Values() ([]any, error)                       { return nil, nil }
func (r *countDeviceRows) RawValues() [][]byte                          { return nil }
func (r *countDeviceRows) Conn() *pgx.Conn                              { return nil }

// intDeviceRow is a pgx.Row that scans a single int (the Count total) or a
// pre-configured error.
type intDeviceRow struct {
	n   int
	err error
}

func (r *intDeviceRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	*dest[0].(*int) = r.n
	return nil
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

// --- CountsByUsers ---

// TestPostgresStore_CountsByUsers_Empty short-circuits an empty user set with no
// query and an empty, non-nil map.
func TestPostgresStore_CountsByUsers_Empty(t *testing.T) {
	t.Parallel()
	fq := &fakeQuerier{}
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

// TestPostgresStore_CountsByUsers_PropagatesQueryError wraps a Query error.
func TestPostgresStore_CountsByUsers_PropagatesQueryError(t *testing.T) {
	t.Parallel()
	boom := errors.New("counts boom")
	fq := &fakeQuerier{queryErr: boom}
	store := NewPostgresStore(fq)

	if _, err := store.CountsByUsers(context.Background(), []string{"auth0|u1"}); !errors.Is(err, boom) {
		t.Fatalf("got %v, want wrapped %v", err, boom)
	}
}

// TestPostgresStore_CountsByUsers_MapsPerUser maps one live-token count per user;
// a user absent from the grouped result is omitted (defaults to 0 at the call site).
func TestPostgresStore_CountsByUsers_MapsPerUser(t *testing.T) {
	t.Parallel()
	fq := &fakeQuerier{queryRows: &countDeviceRows{rows: [][2]any{
		{"auth0|u1", 3},
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
	if got["auth0|u1"] != 3 || got["auth0|u2"] != 1 {
		t.Errorf("counts: got %+v, want {u1:3 u2:1}", got)
	}
	if _, ok := got["auth0|u3"]; ok {
		t.Error("auth0|u3 must be absent (defaults to zero at the call site)")
	}
}

// --- Count ---

// TestPostgresStore_Count_ReturnsTotal returns the scalar count(*).
func TestPostgresStore_Count_ReturnsTotal(t *testing.T) {
	t.Parallel()
	fq := &fakeQuerier{countRow: &intDeviceRow{n: 7}}
	store := NewPostgresStore(fq)

	got, err := store.Count(context.Background())
	if err != nil {
		t.Fatalf("Count: %v", err)
	}
	if got != 7 {
		t.Errorf("Count = %d, want 7", got)
	}
}

// TestPostgresStore_Count_PropagatesError wraps a scan error.
func TestPostgresStore_Count_PropagatesError(t *testing.T) {
	t.Parallel()
	boom := errors.New("count boom")
	fq := &fakeQuerier{countRow: &intDeviceRow{err: boom}}
	store := NewPostgresStore(fq)

	if _, err := store.Count(context.Background()); !errors.Is(err, boom) {
		t.Fatalf("got %v, want wrapped %v", err, boom)
	}
}
