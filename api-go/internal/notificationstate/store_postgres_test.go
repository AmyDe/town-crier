package notificationstate

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// --- fakes ---

type fakeNSQuerier struct {
	execResponses  []execResp
	execIdx        int
	queryResponses []queryResp
	queryIdx       int
	recordedSQL    []string
}

type execResp struct {
	tag pgconn.CommandTag
	err error
}

type queryResp struct {
	rows pgx.Rows
	err  error
}

func (f *fakeNSQuerier) Exec(_ context.Context, sql string, _ ...any) (pgconn.CommandTag, error) {
	f.recordedSQL = append(f.recordedSQL, sql)
	if f.execIdx >= len(f.execResponses) {
		return pgconn.NewCommandTag("EXEC"), nil
	}
	r := f.execResponses[f.execIdx]
	f.execIdx++
	return r.tag, r.err
}

func (f *fakeNSQuerier) Query(_ context.Context, sql string, _ ...any) (pgx.Rows, error) {
	f.recordedSQL = append(f.recordedSQL, sql)
	if f.queryIdx >= len(f.queryResponses) {
		return &fakeNSRows{}, nil
	}
	r := f.queryResponses[f.queryIdx]
	f.queryIdx++
	return r.rows, r.err
}

// fakeNSRows is a minimal pgx.Rows backed by pre-baked scan values.
type fakeNSRows struct {
	rows [][]any
	idx  int
	err  error
}

func newNSRows(rows [][]any) *fakeNSRows { return &fakeNSRows{rows: rows} }
func errNSRows(err error) *fakeNSRows    { return &fakeNSRows{err: err} }

func (r *fakeNSRows) Next() bool {
	if r.err != nil {
		return false
	}
	return r.idx < len(r.rows)
}

func (r *fakeNSRows) Scan(dest ...any) error {
	if r.idx >= len(r.rows) {
		return errors.New("no more rows")
	}
	src := r.rows[r.idx]
	r.idx++
	if len(src) != len(dest) {
		return errors.New("scan: column count mismatch")
	}
	for i, d := range dest {
		switch p := d.(type) {
		case *string:
			if v, ok := src[i].(string); ok {
				*p = v
			}
		case *int:
			if v, ok := src[i].(int); ok {
				*p = v
			}
		case *int64:
			if v, ok := src[i].(int64); ok {
				*p = v
			}
		case *time.Time:
			if v, ok := src[i].(time.Time); ok {
				*p = v
			}
		}
	}
	return nil
}

func (r *fakeNSRows) Close()                                       {}
func (r *fakeNSRows) Err() error                                   { return r.err }
func (r *fakeNSRows) CommandTag() pgconn.CommandTag                { return pgconn.CommandTag{} }
func (r *fakeNSRows) FieldDescriptions() []pgconn.FieldDescription { return nil }
func (r *fakeNSRows) RawValues() [][]byte                          { return nil }
func (r *fakeNSRows) Values() ([]any, error)                       { return nil, nil }
func (r *fakeNSRows) Conn() *pgx.Conn                              { return nil }

// --- Get ---

func TestPostgresNSStore_Get_NotFound(t *testing.T) {
	t.Parallel()
	q := &fakeNSQuerier{queryResponses: []queryResp{{rows: newNSRows(nil)}}}
	s := NewPostgresStore(q)

	got, err := s.Get(context.Background(), "user-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != nil {
		t.Errorf("expected nil (first-touch), got %+v", got)
	}
}

func TestPostgresNSStore_Get_Found(t *testing.T) {
	t.Parallel()
	lastReadAt := time.Date(2026, 6, 26, 10, 0, 0, 0, time.UTC)
	q := &fakeNSQuerier{
		queryResponses: []queryResp{
			{rows: newNSRows([][]any{
				// user_id, last_read_at, version
				{"user-1", lastReadAt, 3},
			})},
		},
	}
	s := NewPostgresStore(q)

	got, err := s.Get(context.Background(), "user-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got == nil {
		t.Fatal("expected non-nil state, got nil")
	}
	if got.UserID != "user-1" {
		t.Errorf("UserID: got %q, want user-1", got.UserID)
	}
	if !got.LastReadAt.Equal(lastReadAt) {
		t.Errorf("LastReadAt: got %v, want %v", got.LastReadAt, lastReadAt)
	}
	if got.Version != 3 {
		t.Errorf("Version: got %d, want 3", got.Version)
	}
}

func TestPostgresNSStore_Get_PropagatesQueryError(t *testing.T) {
	t.Parallel()
	boom := errors.New("get boom")
	q := &fakeNSQuerier{queryResponses: []queryResp{{err: boom}}}
	s := NewPostgresStore(q)

	_, err := s.Get(context.Background(), "user-1")
	if !errors.Is(err, boom) {
		t.Fatalf("got %v, want wrapped %v", err, boom)
	}
}

// --- Save ---

func TestPostgresNSStore_Save_Success(t *testing.T) {
	t.Parallel()
	q := &fakeNSQuerier{execResponses: []execResp{{tag: pgconn.NewCommandTag("INSERT 0 1")}}}
	s := NewPostgresStore(q)
	st := State{UserID: "user-1", LastReadAt: time.Now(), Version: 1}

	if err := s.Save(context.Background(), st); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(q.recordedSQL) != 1 {
		t.Errorf("expected 1 SQL call, got %d", len(q.recordedSQL))
	}
}

func TestPostgresNSStore_Save_PropagatesExecError(t *testing.T) {
	t.Parallel()
	boom := errors.New("save boom")
	q := &fakeNSQuerier{execResponses: []execResp{{err: boom}}}
	s := NewPostgresStore(q)

	err := s.Save(context.Background(), State{UserID: "user-1", LastReadAt: time.Now()})
	if !errors.Is(err, boom) {
		t.Fatalf("got %v, want wrapped %v", err, boom)
	}
}

// --- UnreadCount ---

func TestPostgresNSStore_UnreadCount_ReturnsCount(t *testing.T) {
	t.Parallel()
	q := &fakeNSQuerier{
		queryResponses: []queryResp{
			{rows: newNSRows([][]any{
				{int64(7)},
			})},
		},
	}
	s := NewPostgresStore(q)

	count, err := s.UnreadCount(context.Background(), "user-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if count != 7 {
		t.Errorf("count: got %d, want 7", count)
	}
}

func TestPostgresNSStore_UnreadCount_PropagatesQueryError(t *testing.T) {
	t.Parallel()
	boom := errors.New("count boom")
	q := &fakeNSQuerier{queryResponses: []queryResp{{err: boom}}}
	s := NewPostgresStore(q)

	_, err := s.UnreadCount(context.Background(), "user-1")
	if !errors.Is(err, boom) {
		t.Fatalf("got %v, want wrapped %v", err, boom)
	}
}

func TestPostgresNSStore_UnreadCount_ZeroWhenNoRows(t *testing.T) {
	t.Parallel()
	// An empty result from the count query means 0 unread.
	q := &fakeNSQuerier{queryResponses: []queryResp{{rows: newNSRows(nil)}}}
	s := NewPostgresStore(q)

	count, err := s.UnreadCount(context.Background(), "user-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if count != 0 {
		t.Errorf("count: got %d, want 0", count)
	}
}

// --- MarkAllRead / MarkApplicationsRead (count parsing + error propagation) ---

func TestPostgresNSStore_MarkAllRead_ReturnsClearedCount(t *testing.T) {
	t.Parallel()
	q := &fakeNSQuerier{queryResponses: []queryResp{{rows: newNSRows([][]any{{int64(4)}})}}}
	s := NewPostgresStore(q)

	cleared, err := s.MarkAllRead(context.Background(), "user-1", time.Now())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cleared != 4 {
		t.Errorf("cleared: got %d, want 4", cleared)
	}
}

func TestPostgresNSStore_MarkAllRead_PropagatesQueryError(t *testing.T) {
	t.Parallel()
	boom := errors.New("mark-all boom")
	q := &fakeNSQuerier{queryResponses: []queryResp{{err: boom}}}
	s := NewPostgresStore(q)

	_, err := s.MarkAllRead(context.Background(), "user-1", time.Now())
	if !errors.Is(err, boom) {
		t.Fatalf("got %v, want wrapped %v", err, boom)
	}
}

func TestPostgresNSStore_MarkApplicationsRead_ReturnsClearedCount(t *testing.T) {
	t.Parallel()
	q := &fakeNSQuerier{queryResponses: []queryResp{{rows: newNSRows([][]any{{int64(1)}})}}}
	s := NewPostgresStore(q)

	cleared, err := s.MarkApplicationsRead(context.Background(), "user-1",
		[]string{"24-01234"}, []int{330}, time.Now())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cleared != 1 {
		t.Errorf("cleared: got %d, want 1", cleared)
	}
}

func TestPostgresNSStore_MarkApplicationsRead_PropagatesQueryError(t *testing.T) {
	t.Parallel()
	boom := errors.New("mark-read boom")
	q := &fakeNSQuerier{queryResponses: []queryResp{{err: boom}}}
	s := NewPostgresStore(q)

	_, err := s.MarkApplicationsRead(context.Background(), "user-1",
		[]string{"24-01234"}, []int{330}, time.Now())
	if !errors.Is(err, boom) {
		t.Fatalf("got %v, want wrapped %v", err, boom)
	}
}

// --- MarkReadUpTo (temporary advance compat shim, tc-ekii) ---

func TestPostgresNSStore_MarkReadUpTo_ReturnsClearedCount(t *testing.T) {
	t.Parallel()
	q := &fakeNSQuerier{queryResponses: []queryResp{{rows: newNSRows([][]any{{int64(2)}})}}}
	s := NewPostgresStore(q)

	cleared, err := s.MarkReadUpTo(context.Background(), "user-1", time.Now(), time.Now())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cleared != 2 {
		t.Errorf("cleared: got %d, want 2", cleared)
	}
}

func TestPostgresNSStore_MarkReadUpTo_PropagatesQueryError(t *testing.T) {
	t.Parallel()
	boom := errors.New("mark-up-to boom")
	q := &fakeNSQuerier{queryResponses: []queryResp{{err: boom}}}
	s := NewPostgresStore(q)

	_, err := s.MarkReadUpTo(context.Background(), "user-1", time.Now(), time.Now())
	if !errors.Is(err, boom) {
		t.Fatalf("got %v, want wrapped %v", err, boom)
	}
}

// --- DeleteByUserID ---

func TestPostgresNSStore_DeleteByUserID_Success(t *testing.T) {
	t.Parallel()
	q := &fakeNSQuerier{execResponses: []execResp{{tag: pgconn.NewCommandTag("DELETE 1")}}}
	s := NewPostgresStore(q)

	if err := s.DeleteByUserID(context.Background(), "user-1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestPostgresNSStore_DeleteByUserID_PropagatesExecError(t *testing.T) {
	t.Parallel()
	boom := errors.New("del boom")
	q := &fakeNSQuerier{execResponses: []execResp{{err: boom}}}
	s := NewPostgresStore(q)

	err := s.DeleteByUserID(context.Background(), "user-1")
	if !errors.Is(err, boom) {
		t.Fatalf("got %v, want wrapped %v", err, boom)
	}
}
