package notifications

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// --- fakes ---

// fakeQuerier implements querier using pre-configured responses. A single
// Exec error or Query/scan configuration is set per test; multiple calls
// can be sequenced via a slice of responses.
type fakeQuerier struct {
	// execResponses is consumed in order, one per Exec call.
	execResponses []execResponse
	execIdx       int
	// queryResponses is consumed in order, one per Query call.
	queryResponses []queryResponse
	queryIdx       int
	// recordedSQL captures every SQL statement issued.
	recordedSQL []string
}

type execResponse struct {
	tag pgconn.CommandTag
	err error
}

type queryResponse struct {
	rows pgx.Rows
	err  error
}

func (f *fakeQuerier) Exec(_ context.Context, sql string, _ ...any) (pgconn.CommandTag, error) {
	f.recordedSQL = append(f.recordedSQL, sql)
	if f.execIdx >= len(f.execResponses) {
		return pgconn.NewCommandTag("EXEC"), nil
	}
	r := f.execResponses[f.execIdx]
	f.execIdx++
	return r.tag, r.err
}

func (f *fakeQuerier) Query(_ context.Context, sql string, _ ...any) (pgx.Rows, error) {
	f.recordedSQL = append(f.recordedSQL, sql)
	if f.queryIdx >= len(f.queryResponses) {
		return &fakeRows{}, nil
	}
	r := f.queryResponses[f.queryIdx]
	f.queryIdx++
	return r.rows, r.err
}

// fakeRows is a minimal pgx.Rows implementation backed by pre-baked scan values.
// Each element of rows is a []any slice that Scan() copies into the dest pointers.
type fakeRows struct {
	rows [][]any
	idx  int
	err  error
}

func newFakeRows(rows [][]any) *fakeRows { return &fakeRows{rows: rows} }
func errRows(err error) *fakeRows        { return &fakeRows{err: err} }

func (r *fakeRows) Next() bool {
	if r.err != nil {
		return false
	}
	return r.idx < len(r.rows)
}

func (r *fakeRows) Scan(dest ...any) error {
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
		case **string:
			switch v := src[i].(type) {
			case nil:
				*p = nil
			case string:
				s := v
				*p = &s
			case *string:
				*p = v
			}
		case *bool:
			if v, ok := src[i].(bool); ok {
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

func (r *fakeRows) Close()                                       {}
func (r *fakeRows) Err() error                                   { return r.err }
func (r *fakeRows) CommandTag() pgconn.CommandTag                { return pgconn.CommandTag{} }
func (r *fakeRows) FieldDescriptions() []pgconn.FieldDescription { return nil }
func (r *fakeRows) RawValues() [][]byte                          { return nil }
func (r *fakeRows) Values() ([]any, error)                       { return nil, nil }
func (r *fakeRows) Conn() *pgx.Conn                              { return nil }

// --- helpers ---

func newTestNotification() DigestNotification {
	return DigestNotification{
		ID:                     "notif-id-1",
		UserID:                 "user-1",
		ApplicationUID:         "uid-A",
		ApplicationName:        "22/0001",
		ApplicationAddress:     "1 Test Street",
		ApplicationDescription: "outline planning permission",
		AuthorityID:            100,
		EventType:              EventNewApplication,
		Sources:                "Zone",
		CreatedAt:              time.Date(2026, 6, 26, 12, 0, 0, 0, time.UTC),
	}
}

// --- Create ---

func TestPostgresStore_Create_Success(t *testing.T) {
	t.Parallel()
	q := &fakeQuerier{
		execResponses: []execResponse{{tag: pgconn.NewCommandTag("INSERT 0 1")}},
	}
	s := NewPostgresStore(q)
	n := newTestNotification()

	if err := s.Create(context.Background(), n); err != nil {
		t.Fatalf("Create: unexpected error: %v", err)
	}
	if len(q.recordedSQL) != 1 {
		t.Fatalf("Create: expected 1 SQL call, got %d", len(q.recordedSQL))
	}
}

func TestPostgresStore_Create_PropagatesExecError(t *testing.T) {
	t.Parallel()
	boom := errors.New("db boom")
	q := &fakeQuerier{
		execResponses: []execResponse{{err: boom}},
	}
	s := NewPostgresStore(q)

	err := s.Create(context.Background(), newTestNotification())
	if !errors.Is(err, boom) {
		t.Fatalf("Create: got err %v, want wrapped %v", err, boom)
	}
}

// --- GetLatestUnreadByApplications ---

func TestPostgresStore_GetLatestUnreadByApplications_EmptyUIDs(t *testing.T) {
	t.Parallel()
	q := &fakeQuerier{}
	s := NewPostgresStore(q)

	got, err := s.GetLatestUnreadByApplications(context.Background(), "user-1", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got == nil || len(got) != 0 {
		t.Errorf("empty UIDs: got %v, want empty non-nil map", got)
	}
	if len(q.recordedSQL) != 0 {
		t.Error("empty UIDs must not issue any SQL")
	}
}

func TestPostgresStore_GetLatestUnreadByApplications_PropagatesQueryError(t *testing.T) {
	t.Parallel()
	boom := errors.New("query boom")
	q := &fakeQuerier{
		queryResponses: []queryResponse{{err: boom}},
	}
	s := NewPostgresStore(q)

	_, err := s.GetLatestUnreadByApplications(
		context.Background(), "user-1", []string{"uid-A"})
	if !errors.Is(err, boom) {
		t.Fatalf("got err %v, want wrapped %v", err, boom)
	}
}

func TestPostgresStore_GetLatestUnreadByApplications_ReturnsLatestPerUID(t *testing.T) {
	t.Parallel()
	// DISTINCT ON returns one row per application_uid (the newest). The fake
	// returns 2 rows — one per uid — matching the 4-column projection:
	// application_uid, decision, event_type, created_at.
	newerCreatedAt := time.Date(2026, 2, 1, 10, 0, 0, 0, time.UTC)
	uidBCreatedAt := time.Date(2026, 1, 15, 8, 0, 0, 0, time.UTC)
	q := &fakeQuerier{
		queryResponses: []queryResponse{
			{rows: newFakeRows([][]any{
				// application_uid, decision, event_type, created_at
				{"uid-A", "Permitted", string(EventDecisionUpdate), newerCreatedAt},
				{"uid-B", nil, string(EventNewApplication), uidBCreatedAt},
			})},
		},
	}
	s := NewPostgresStore(q)

	got, err := s.GetLatestUnreadByApplications(
		context.Background(), "user-1", []string{"uid-A", "uid-B"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("map size: got %d, want 2", len(got))
	}
	a := got["uid-A"]
	if a.EventType != EventDecisionUpdate {
		t.Errorf("uid-A event type: got %q, want DecisionUpdate", a.EventType)
	}
	if a.Decision == nil || *a.Decision != "Permitted" {
		t.Errorf("uid-A decision: got %v, want 'Permitted'", a.Decision)
	}
	if !a.CreatedAt.Equal(newerCreatedAt) {
		t.Errorf("uid-A createdAt: got %v, want %v", a.CreatedAt, newerCreatedAt)
	}
	b := got["uid-B"]
	if b.EventType != EventNewApplication {
		t.Errorf("uid-B event type: got %q, want NewApplication", b.EventType)
	}
}

// --- GetByUserAndApplication ---

func TestPostgresStore_GetByUserAndApplication_NotFound(t *testing.T) {
	t.Parallel()
	q := &fakeQuerier{
		queryResponses: []queryResponse{{rows: newFakeRows(nil)}},
	}
	s := NewPostgresStore(q)

	got, err := s.GetByUserAndApplication(context.Background(), "user-1", "uid-A", 100, EventNewApplication)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != nil {
		t.Errorf("expected nil, got %+v", got)
	}
}

func TestPostgresStore_GetByUserAndApplication_Found(t *testing.T) {
	t.Parallel()
	createdAt := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	q := &fakeQuerier{
		queryResponses: []queryResponse{
			{rows: newFakeRows([][]any{
				{"notif-id-1", "user-1", "uid-A", "22/0001", nil, "", "", nil,
					100, nil, string(EventNewApplication), "", false, false, createdAt},
			})},
		},
	}
	s := NewPostgresStore(q)

	got, err := s.GetByUserAndApplication(context.Background(), "user-1", "uid-A", 100, EventNewApplication)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got == nil {
		t.Fatal("expected non-nil notification, got nil")
	}
	if got.ID != "notif-id-1" {
		t.Errorf("ID: got %q, want notif-id-1", got.ID)
	}
}

// --- ByUserSince / AllByUser ---

func TestPostgresStore_ByUserSince_PropagatesQueryError(t *testing.T) {
	t.Parallel()
	boom := errors.New("since boom")
	q := &fakeQuerier{queryResponses: []queryResponse{{err: boom}}}
	s := NewPostgresStore(q)

	_, err := s.ByUserSince(context.Background(), "user-1", time.Now())
	if !errors.Is(err, boom) {
		t.Fatalf("got %v, want wrapped %v", err, boom)
	}
}

func TestPostgresStore_ByUserSince_ReturnsEmptySliceWhenNone(t *testing.T) {
	t.Parallel()
	q := &fakeQuerier{queryResponses: []queryResponse{{rows: newFakeRows(nil)}}}
	s := NewPostgresStore(q)

	got, err := s.ByUserSince(context.Background(), "user-1", time.Now())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got == nil || len(got) != 0 {
		t.Errorf("want empty non-nil slice, got %v", got)
	}
}

func TestPostgresStore_AllByUser_PropagatesQueryError(t *testing.T) {
	t.Parallel()
	boom := errors.New("all boom")
	q := &fakeQuerier{queryResponses: []queryResponse{{err: boom}}}
	s := NewPostgresStore(q)

	_, err := s.AllByUser(context.Background(), "user-1")
	if !errors.Is(err, boom) {
		t.Fatalf("got %v, want wrapped %v", err, boom)
	}
}

// --- UnsentEmailsByUser / UserIDsWithUnsentEmails ---

func TestPostgresStore_UnsentEmailsByUser_PropagatesQueryError(t *testing.T) {
	t.Parallel()
	boom := errors.New("unsent boom")
	q := &fakeQuerier{queryResponses: []queryResponse{{err: boom}}}
	s := NewPostgresStore(q)

	_, err := s.UnsentEmailsByUser(context.Background(), "user-1")
	if !errors.Is(err, boom) {
		t.Fatalf("got %v, want wrapped %v", err, boom)
	}
}

func TestPostgresStore_UserIDsWithUnsentEmails_DeduplicatesIDs(t *testing.T) {
	t.Parallel()
	q := &fakeQuerier{
		queryResponses: []queryResponse{
			{rows: newFakeRows([][]any{
				{"user-A"},
				{"user-B"},
				{"user-A"}, // duplicate — should be deduped
			})},
		},
	}
	s := NewPostgresStore(q)

	got, err := s.UserIDsWithUnsentEmails(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("want 2 unique user IDs, got %d: %v", len(got), got)
	}
}

func TestPostgresStore_UserIDsWithUnsentEmails_PropagatesQueryError(t *testing.T) {
	t.Parallel()
	boom := errors.New("ids boom")
	q := &fakeQuerier{queryResponses: []queryResponse{{err: boom}}}
	s := NewPostgresStore(q)

	_, err := s.UserIDsWithUnsentEmails(context.Background())
	if !errors.Is(err, boom) {
		t.Fatalf("got %v, want wrapped %v", err, boom)
	}
}

// --- CountsByUsers ---

func TestPostgresStore_CountsByUsers_EmptyUsers(t *testing.T) {
	t.Parallel()
	q := &fakeQuerier{}
	s := NewPostgresStore(q)

	got, err := s.CountsByUsers(context.Background(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got == nil || len(got) != 0 {
		t.Errorf("empty users: got %v, want empty non-nil map", got)
	}
	if len(q.recordedSQL) != 0 {
		t.Error("empty users must not issue any SQL")
	}
}

func TestPostgresStore_CountsByUsers_PropagatesQueryError(t *testing.T) {
	t.Parallel()
	boom := errors.New("counts boom")
	q := &fakeQuerier{queryResponses: []queryResponse{{err: boom}}}
	s := NewPostgresStore(q)

	_, err := s.CountsByUsers(context.Background(), []string{"user-1"})
	if !errors.Is(err, boom) {
		t.Fatalf("got %v, want wrapped %v", err, boom)
	}
}

func TestPostgresStore_CountsByUsers_MapsPerUser(t *testing.T) {
	t.Parallel()
	// The GROUP BY query returns one row per user that HAS notifications.
	// user-C is queried but absent from the result — the caller defaults it to
	// {0, 0} via the Go map zero value, so it must NOT appear in the map.
	q := &fakeQuerier{
		queryResponses: []queryResponse{
			{rows: newFakeRows([][]any{
				// user_id, total, unread
				{"user-A", 57, 2},
				{"user-B", 3, 0},
			})},
		},
	}
	s := NewPostgresStore(q)

	got, err := s.CountsByUsers(context.Background(), []string{"user-A", "user-B", "user-C"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("map size: got %d, want 2 (absent users omitted)", len(got))
	}
	if got["user-A"] != (NotificationCounts{Total: 57, Unread: 2}) {
		t.Errorf("user-A: got %+v, want {57 2}", got["user-A"])
	}
	if got["user-B"] != (NotificationCounts{Total: 3, Unread: 0}) {
		t.Errorf("user-B: got %+v, want {3 0}", got["user-B"])
	}
	if _, ok := got["user-C"]; ok {
		t.Errorf("user-C must be absent (defaults to zero at the call site)")
	}
}

// --- MarkEmailSent ---

func TestPostgresStore_MarkEmailSent_PropagatesExecError(t *testing.T) {
	t.Parallel()
	boom := errors.New("mark boom")
	q := &fakeQuerier{execResponses: []execResponse{{err: boom}}}
	s := NewPostgresStore(q)

	err := s.MarkEmailSent(context.Background(), newTestNotification())
	if !errors.Is(err, boom) {
		t.Fatalf("got %v, want wrapped %v", err, boom)
	}
}

// --- DeleteAllByUserID ---

func TestPostgresStore_DeleteAllByUserID_Success(t *testing.T) {
	t.Parallel()
	q := &fakeQuerier{execResponses: []execResponse{{tag: pgconn.NewCommandTag("DELETE 5")}}}
	s := NewPostgresStore(q)

	if err := s.DeleteAllByUserID(context.Background(), "user-1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestPostgresStore_DeleteAllByUserID_PropagatesExecError(t *testing.T) {
	t.Parallel()
	boom := errors.New("delete boom")
	q := &fakeQuerier{execResponses: []execResponse{{err: boom}}}
	s := NewPostgresStore(q)

	err := s.DeleteAllByUserID(context.Background(), "user-1")
	if !errors.Is(err, boom) {
		t.Fatalf("got %v, want wrapped %v", err, boom)
	}
}

// --- PurgeOlderThan ---

func TestPostgresStore_PurgeOlderThan_ReturnsRowsDeleted(t *testing.T) {
	t.Parallel()
	q := &fakeQuerier{
		execResponses: []execResponse{{tag: pgconn.NewCommandTag("DELETE 42")}},
	}
	s := NewPostgresStore(q)

	n, err := s.PurgeOlderThan(context.Background(), time.Now())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n != 42 {
		t.Errorf("rows deleted: got %d, want 42", n)
	}
}

func TestPostgresStore_PurgeOlderThan_PropagatesExecError(t *testing.T) {
	t.Parallel()
	boom := errors.New("purge boom")
	q := &fakeQuerier{execResponses: []execResponse{{err: boom}}}
	s := NewPostgresStore(q)

	_, err := s.PurgeOlderThan(context.Background(), time.Now())
	if !errors.Is(err, boom) {
		t.Fatalf("got %v, want wrapped %v", err, boom)
	}
}
