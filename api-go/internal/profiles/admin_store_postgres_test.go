package profiles

import (
	"context"
	"encoding/base64"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// fakeUserRows is a pgx.Rows over pre-baked user-column slices. Each element is
// a full userSelectCols projection; Scan copies it via fakeScanRow so any
// column-order drift is caught here as well as in scanUserRow's own test.
type fakeUserRows struct {
	rows [][]any
	idx  int
	err  error
}

func (r *fakeUserRows) Next() bool {
	if r.err != nil {
		return false
	}
	return r.idx < len(r.rows)
}

func (r *fakeUserRows) Scan(dest ...any) error {
	sr := &fakeScanRow{cols: r.rows[r.idx]}
	r.idx++
	return sr.Scan(dest...)
}

func (r *fakeUserRows) Close()                                       {}
func (r *fakeUserRows) Err() error                                   { return r.err }
func (r *fakeUserRows) CommandTag() pgconn.CommandTag                { return pgconn.CommandTag{} }
func (r *fakeUserRows) FieldDescriptions() []pgconn.FieldDescription { return nil }
func (r *fakeUserRows) RawValues() [][]byte                          { return nil }
func (r *fakeUserRows) Values() ([]any, error)                       { return nil, nil }
func (r *fakeUserRows) Conn() *pgx.Conn                              { return nil }

// recordingQuerier captures the SQL and args of the single Query it serves and
// returns pre-baked rows. Exec/QueryRow panic — List only uses Query.
type recordingQuerier struct {
	sql  string
	args []any
	rows *fakeUserRows
	err  error
}

func (q *recordingQuerier) Exec(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
	panic("recordingQuerier.Exec not expected")
}

func (q *recordingQuerier) Query(_ context.Context, sql string, args ...any) (pgx.Rows, error) {
	q.sql = sql
	q.args = args
	if q.err != nil {
		return nil, q.err
	}
	return q.rows, nil
}

func (q *recordingQuerier) QueryRow(_ context.Context, _ string, _ ...any) pgx.Row {
	panic("recordingQuerier.QueryRow not expected")
}

// userRow builds one full userSelectCols projection for a user with the given
// id and created_at. Order MUST match userSelectCols / scanUserRow.
func userRow(userID string, createdAt time.Time) []any {
	return []any{
		// user_id, email, push_enabled, digest_day,
		userID, nil, true, 1,
		// email_digest_enabled, saved_decision_push, saved_decision_email,
		true, true, true,
		// zone_preferences::text,
		"{}",
		// tier, subscription_expiry, original_transaction_id, grace_period_expiry,
		"Free", nil, nil, nil,
		// last_active_at, last_active_at_epoch, created_at, watch_zone_count, version
		createdAt, createdAt.UnixMilli(), createdAt, nil, 0,
	}
}

func TestEncodeDecodeListCursor_RoundTrip(t *testing.T) {
	t.Parallel()
	created := time.Date(2026, 1, 2, 3, 4, 5, 123456789, time.UTC)
	// User IDs legitimately contain "|" (auth0|..., apple|...); the cursor must
	// survive that since created_at is encoded first and split off with SplitN.
	token := encodeListCursor(created, "auth0|u1")
	got, err := decodeListCursor(token)
	if err != nil {
		t.Fatalf("decodeListCursor: %v", err)
	}
	if !got.CreatedAt.Equal(created) {
		t.Errorf("CreatedAt: got %v, want %v", got.CreatedAt, created)
	}
	if got.LastUserID != "auth0|u1" {
		t.Errorf("LastUserID: got %q, want auth0|u1", got.LastUserID)
	}
}

// encodeRaw base64url-encodes an arbitrary payload so malformed-cursor cases can
// smuggle a payload that decodeListCursor will reject on the "|" split.
func encodeRaw(raw string) string {
	return base64.RawURLEncoding.EncodeToString([]byte(raw))
}

func TestDecodeListCursor_Malformed(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		token string
	}{
		{"not base64url", "!!!not-base64!!!"},
		{"no separator", encodeRaw("justonefield")},
		{"bad timestamp", encodeRaw("not-a-time|auth0|u1")},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if _, err := decodeListCursor(tc.token); err == nil {
				t.Errorf("decodeListCursor(%q): want error, got nil", tc.token)
			}
		})
	}
}

func TestList_FirstPage_OrdersByCreatedAtUserID(t *testing.T) {
	t.Parallel()
	t0 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	q := &recordingQuerier{rows: &fakeUserRows{rows: [][]any{userRow("a", t0), userRow("b", t0)}}}
	store := NewPostgresAdminStore(q)

	page, err := store.List(context.Background(), "", 5, "")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if !strings.Contains(q.sql, "ORDER BY created_at, user_id") {
		t.Errorf("SQL missing compound ORDER BY: %s", q.sql)
	}
	if strings.Contains(q.sql, "(created_at, user_id) >") {
		t.Errorf("first page must not carry a cursor guard: %s", q.sql)
	}
	if len(page.Profiles) != 2 {
		t.Fatalf("profiles: got %d, want 2", len(page.Profiles))
	}
	if page.ContinuationToken != "" {
		t.Errorf("undersized page must not emit a token, got %q", page.ContinuationToken)
	}
}

func TestList_FullPage_EmitsCompoundCursor(t *testing.T) {
	t.Parallel()
	t0 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	t1 := t0.Add(time.Hour)
	q := &recordingQuerier{rows: &fakeUserRows{rows: [][]any{userRow("a", t0), userRow("b", t1)}}}
	store := NewPostgresAdminStore(q)

	page, err := store.List(context.Background(), "", 2, "")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if page.ContinuationToken == "" {
		t.Fatal("full page must emit a continuation token")
	}
	cur, err := decodeListCursor(page.ContinuationToken)
	if err != nil {
		t.Fatalf("decode emitted token: %v", err)
	}
	if !cur.CreatedAt.Equal(t1) || cur.LastUserID != "b" {
		t.Errorf("cursor = {%v, %q}, want {%v, b}", cur.CreatedAt, cur.LastUserID, t1)
	}
}

func TestList_WithCursor_AppliesCompoundGuard(t *testing.T) {
	t.Parallel()
	t0 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	q := &recordingQuerier{rows: &fakeUserRows{}}
	store := NewPostgresAdminStore(q)

	if _, err := store.List(context.Background(), "", 5, encodeListCursor(t0, "a")); err != nil {
		t.Fatalf("List: %v", err)
	}
	if !strings.Contains(q.sql, "(created_at, user_id) > ($1, $2)") {
		t.Errorf("SQL missing compound guard: %s", q.sql)
	}
	if !strings.Contains(q.sql, "ORDER BY created_at, user_id") {
		t.Errorf("SQL missing compound ORDER BY: %s", q.sql)
	}
	if len(q.args) != 3 {
		t.Fatalf("args: got %d, want 3", len(q.args))
	}
	if got, ok := q.args[0].(time.Time); !ok || !got.Equal(t0) {
		t.Errorf("args[0] = %v, want createdAt %v", q.args[0], t0)
	}
	if q.args[1] != "a" {
		t.Errorf("args[1] = %v, want user id \"a\"", q.args[1])
	}
}

func TestList_EmailAndCursor_Branch(t *testing.T) {
	t.Parallel()
	t0 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	q := &recordingQuerier{rows: &fakeUserRows{}}
	store := NewPostgresAdminStore(q)

	if _, err := store.List(context.Background(), "bob", 5, encodeListCursor(t0, "a")); err != nil {
		t.Fatalf("List: %v", err)
	}
	if !strings.Contains(q.sql, "email ILIKE $1 AND (created_at, user_id) > ($2, $3)") {
		t.Errorf("SQL missing email + compound guard: %s", q.sql)
	}
	if len(q.args) != 4 {
		t.Fatalf("args: got %d, want 4", len(q.args))
	}
	if q.args[0] != "%bob%" {
		t.Errorf("args[0] = %v, want %%bob%%", q.args[0])
	}
}

func TestList_BadCursor_ReturnsError(t *testing.T) {
	t.Parallel()
	store := NewPostgresAdminStore(&recordingQuerier{rows: &fakeUserRows{}})
	if _, err := store.List(context.Background(), "", 5, "!!!not-base64!!!"); err == nil {
		t.Fatal("List with malformed cursor: want error, got nil")
	}
}
