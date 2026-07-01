//go:build integration

package notificationstate

import (
	"context"
	"testing"
	"time"

	"github.com/AmyDe/town-crier/api-go/internal/notifications"
	"github.com/AmyDe/town-crier/api-go/internal/platform/postgres/pgtest"
	"github.com/jackc/pgx/v5/pgxpool"
)

// newPGStateStore returns a PostgresStore over a freshly truncated integration-
// test database. Both notification_state and notifications are truncated so
// UnreadCount cross-reads start from a clean slate.
// Integration tests must NOT call t.Parallel (pgtest advisory lock).
func newPGStateStore(t *testing.T) (*PostgresStore, *pgxpool.Pool) {
	t.Helper()
	pool := pgtest.New(t)
	pgtest.Truncate(t, pool, "notification_state", "notifications")
	return NewPostgresStore(pool), pool
}

// Backfill SQL mirrors migrations/0015_notifications_read_at.sql. These integration
// tests seed a pre-migration state (notifications created via the store, all
// read_at IS NULL, plus a watermark row) and replay the backfill to derive read_at,
// then assert read_at IS NULL behaviour matches what the watermark model gave — the
// explicit equivalence acceptance criterion (ADR 0035). Keep in sync with 0015.
const (
	backfillWatermarkedSQL = `
UPDATE notifications n
SET read_at = ns.last_read_at
FROM notification_state ns
WHERE n.user_id = ns.user_id
  AND n.created_at <= ns.last_read_at
  AND n.read_at IS NULL`

	backfillNoWatermarkSQL = `
UPDATE notifications n
SET read_at = n.created_at
WHERE n.read_at IS NULL
  AND NOT EXISTS (SELECT 1 FROM notification_state ns WHERE ns.user_id = n.user_id)`
)

// backfillReadAt replays migration 0015's two backfill UPDATEs, deriving read_at
// from the seeded watermark rows.
func backfillReadAt(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	ctx := context.Background()
	if _, err := pool.Exec(ctx, backfillWatermarkedSQL); err != nil {
		t.Fatalf("backfill watermarked: %v", err)
	}
	if _, err := pool.Exec(ctx, backfillNoWatermarkSQL); err != nil {
		t.Fatalf("backfill no-watermark: %v", err)
	}
}

// seedUnreadNotif inserts one unread notification (Create leaves read_at NULL —
// the pre-backfill state) for the user via the notifications store. The `name`
// argument is written to application_name (= a.Name, the PlanIt case reference
// every client and the push payload carry); application_uid is set to a DISTINCT
// value (name + "/FUL", = a.UID) so the two columns never coincide. This mirrors
// real PlanIt data (name "24/0001", uid "24/0001/FUL") and is load-bearing: it
// makes mark-read fixtures fail loudly if the mutation ever matches
// application_uid again instead of application_name (#733).
func seedUnreadNotif(t *testing.T, pool *pgxpool.Pool, id, userID, name string, authorityID int, createdAt time.Time) {
	t.Helper()
	nStore := notifications.NewPostgresStore(pool)
	n := notifications.DigestNotification{
		ID: id, UserID: userID, ApplicationName: name, ApplicationUID: name + "/FUL",
		AuthorityID: authorityID, EventType: notifications.EventNewApplication,
		Sources: "Zone", CreatedAt: createdAt,
	}
	if err := nStore.Create(context.Background(), n); err != nil {
		t.Fatalf("seed notification %q: %v", id, err)
	}
}

// unreadCountFor reads the raw read_at-IS-NULL count for a user (independent of the
// store method under test).
func unreadCountFor(t *testing.T, pool *pgxpool.Pool, userID string) int {
	t.Helper()
	var n int
	if err := pool.QueryRow(context.Background(),
		"SELECT count(*) FROM notifications WHERE user_id = $1 AND read_at IS NULL", userID).Scan(&n); err != nil {
		t.Fatalf("count unread for %q: %v", userID, err)
	}
	return n
}

// --- Get / Save round-trip ---

func TestIntegration_NotificationState_GetSave(t *testing.T) {
	s, _ := newPGStateStore(t)
	ctx := context.Background()

	// Get on a brand-new user must return nil (first-touch).
	got, err := s.Get(ctx, "user-1")
	if err != nil {
		t.Fatalf("Get (first touch): %v", err)
	}
	if got != nil {
		t.Fatalf("first-touch Get: expected nil, got %+v", got)
	}

	// Save and read back.
	lastReadAt := time.Date(2026, 6, 26, 10, 0, 0, 0, time.UTC)
	st := State{UserID: "user-1", LastReadAt: lastReadAt, Version: 2}
	if err := s.Save(ctx, st); err != nil {
		t.Fatalf("Save: %v", err)
	}

	back, err := s.Get(ctx, "user-1")
	if err != nil {
		t.Fatalf("Get after Save: %v", err)
	}
	if back == nil {
		t.Fatal("Get after Save: expected non-nil state")
	}
	if back.UserID != "user-1" {
		t.Errorf("UserID: got %q, want user-1", back.UserID)
	}
	if !back.LastReadAt.Equal(lastReadAt) {
		t.Errorf("LastReadAt: got %v, want %v", back.LastReadAt, lastReadAt)
	}
	if back.Version != 2 {
		t.Errorf("Version: got %d, want 2", back.Version)
	}
}

func TestIntegration_NotificationState_Save_Idempotent(t *testing.T) {
	s, _ := newPGStateStore(t)
	ctx := context.Background()

	t1 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	t2 := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)

	if err := s.Save(ctx, State{UserID: "user-1", LastReadAt: t1, Version: 1}); err != nil {
		t.Fatalf("first Save: %v", err)
	}
	if err := s.Save(ctx, State{UserID: "user-1", LastReadAt: t2, Version: 2}); err != nil {
		t.Fatalf("second Save: %v", err)
	}

	got, err := s.Get(ctx, "user-1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got == nil {
		t.Fatal("Get: expected non-nil state")
	}
	if !got.LastReadAt.Equal(t2) {
		t.Errorf("LastReadAt after update: got %v, want %v", got.LastReadAt, t2)
	}
	if got.Version != 2 {
		t.Errorf("Version after update: got %d, want 2", got.Version)
	}
}

// --- UnreadCount over read_at IS NULL, equivalent to a watermark fixture ---

// TestIntegration_NotificationState_UnreadCount proves UnreadCount under
// read_at IS NULL equals the count an equivalent watermark fixture (created_at >
// last_read_at, backfilled per migration 0015) would give. Watermark at t1 → the
// two later notifications count; watermark at t3 (the boundary) → nothing counts.
func TestIntegration_NotificationState_UnreadCount(t *testing.T) {
	t1 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	t2 := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)
	t3 := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)

	t.Run("watermark at t1 -> 2 unread", func(t *testing.T) {
		s, pool := newPGStateStore(t)
		ctx := context.Background()
		seedUnreadNotif(t, pool, "n1", "user-1", "uid-A", 100, t1)
		seedUnreadNotif(t, pool, "n2", "user-1", "uid-B", 100, t2)
		seedUnreadNotif(t, pool, "n3", "user-1", "uid-C", 100, t3)
		if err := s.Save(ctx, State{UserID: "user-1", LastReadAt: t1, Version: 1}); err != nil {
			t.Fatalf("seed watermark: %v", err)
		}
		backfillReadAt(t, pool) // read_at = t1 for n1 (created_at <= t1); n2, n3 stay NULL.

		count, err := s.UnreadCount(ctx, "user-1")
		if err != nil {
			t.Fatalf("UnreadCount: %v", err)
		}
		if count != 2 {
			t.Errorf("unread count: got %d, want 2", count)
		}
	})

	t.Run("watermark at t3 (boundary, inclusive) -> 0 unread", func(t *testing.T) {
		s, pool := newPGStateStore(t)
		ctx := context.Background()
		seedUnreadNotif(t, pool, "n1", "user-1", "uid-A", 100, t1)
		seedUnreadNotif(t, pool, "n2", "user-1", "uid-B", 100, t2)
		seedUnreadNotif(t, pool, "n3", "user-1", "uid-C", 100, t3)
		if err := s.Save(ctx, State{UserID: "user-1", LastReadAt: t3, Version: 1}); err != nil {
			t.Fatalf("seed watermark: %v", err)
		}
		backfillReadAt(t, pool) // all created_at <= t3 → all read.

		count, err := s.UnreadCount(ctx, "user-1")
		if err != nil {
			t.Fatalf("UnreadCount: %v", err)
		}
		if count != 0 {
			t.Errorf("unread count at boundary: got %d, want 0", count)
		}
	})
}

// --- MarkAllRead ---

// TestIntegration_NotificationState_MarkAllRead proves mark-all clears every unread
// notification (a subsequent UnreadCount is 0) and bumps the version change token,
// upserting the state row when the user has none.
func TestIntegration_NotificationState_MarkAllRead(t *testing.T) {
	s, pool := newPGStateStore(t)
	ctx := context.Background()
	now := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	t1 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	seedUnreadNotif(t, pool, "n1", "user-1", "uid-A", 100, t1)
	seedUnreadNotif(t, pool, "n2", "user-1", "uid-B", 100, t1)

	cleared, err := s.MarkAllRead(ctx, "user-1", now)
	if err != nil {
		t.Fatalf("MarkAllRead: %v", err)
	}
	if cleared != 2 {
		t.Errorf("cleared: got %d, want 2", cleared)
	}
	if got := unreadCountFor(t, pool, "user-1"); got != 0 {
		t.Errorf("unread after mark-all: got %d, want 0", got)
	}

	// First-touch user: mark-all upserts the state row at version 1.
	st, err := s.Get(ctx, "user-1")
	if err != nil {
		t.Fatalf("Get after mark-all: %v", err)
	}
	if st == nil || st.Version != 1 {
		t.Fatalf("state after first mark-all: got %+v, want version 1", st)
	}

	// A second mark-all (nothing unread) still bumps the version token.
	if _, err := s.MarkAllRead(ctx, "user-1", now); err != nil {
		t.Fatalf("second MarkAllRead: %v", err)
	}
	st, err = s.Get(ctx, "user-1")
	if err != nil {
		t.Fatalf("Get after second mark-all: %v", err)
	}
	if st == nil || st.Version != 2 {
		t.Errorf("version after second mark-all: got %+v, want 2", st)
	}
}

// --- MarkApplicationsRead ---

// TestIntegration_NotificationState_MarkApplicationsRead_MatchesNameNotUID is the
// #733 regression guard. The mutation must scope by application_name (= a.Name, the
// PlanIt case reference every client and the push payload carry), NOT application_uid.
// The seeded row's name and uid differ ("24-01234" vs "24-01234/FUL"): marking by the
// uid must clear nothing, marking by the name must clear the row. Under the old
// application_uid match this was inverted — a silent prod no-op, because the clients
// never send the uid.
func TestIntegration_NotificationState_MarkApplicationsRead_MatchesNameNotUID(t *testing.T) {
	s, pool := newPGStateStore(t)
	ctx := context.Background()
	now := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	t1 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	// name "24-01234", uid "24-01234/FUL" (seedUnreadNotif derives the distinct uid).
	seedUnreadNotif(t, pool, "n1", "user-1", "24-01234", 330, t1)

	// The uid is NOT what the clients send: marking by it must clear nothing.
	clearedByUID, err := s.MarkApplicationsRead(ctx, "user-1", []string{"24-01234/FUL"}, []int{330}, now)
	if err != nil {
		t.Fatalf("MarkApplicationsRead(uid): %v", err)
	}
	if clearedByUID != 0 {
		t.Errorf("marking by uid cleared %d rows, want 0 (match must be on application_name)", clearedByUID)
	}
	if got := readAtOf(t, pool, "n1"); got != nil {
		t.Errorf("row must stay unread after a uid-keyed mark-read, got read_at %v", got)
	}

	// The name (a.Name) is what iOS/web/push carry: marking by it clears the row.
	clearedByName, err := s.MarkApplicationsRead(ctx, "user-1", []string{"24-01234"}, []int{330}, now)
	if err != nil {
		t.Fatalf("MarkApplicationsRead(name): %v", err)
	}
	if clearedByName != 1 {
		t.Errorf("marking by name cleared %d rows, want 1", clearedByName)
	}
	if got := readAtOf(t, pool, "n1"); got == nil {
		t.Error("row must be read (read_at set) after a name-keyed mark-read")
	}
}

// TestIntegration_NotificationState_MarkApplicationsRead_CrossAuthorityGuard proves
// the composite (application_name, authority_id) scoping: two authorities sharing a
// case reference must not cross-contaminate. a.Name is unique within a council but
// collides across councils, so authority_id disambiguates. Marking {name, authority
// 330} read clears only authority 330's row and leaves authority 331's same-name row
// unread.
func TestIntegration_NotificationState_MarkApplicationsRead_CrossAuthorityGuard(t *testing.T) {
	s, pool := newPGStateStore(t)
	ctx := context.Background()
	now := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	t1 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	// Same user, same case reference name "24-01234", two different authorities — both unread.
	seedUnreadNotif(t, pool, "n-330", "user-1", "24-01234", 330, t1)
	seedUnreadNotif(t, pool, "n-331", "user-1", "24-01234", 331, t1)

	cleared, err := s.MarkApplicationsRead(ctx, "user-1", []string{"24-01234"}, []int{330}, now)
	if err != nil {
		t.Fatalf("MarkApplicationsRead: %v", err)
	}
	if cleared != 1 {
		t.Fatalf("cleared: got %d, want 1 (only authority 330)", cleared)
	}
	if got := readAtOf(t, pool, "n-330"); got == nil {
		t.Error("authority 330's row should be read (read_at set)")
	}
	if got := readAtOf(t, pool, "n-331"); got != nil {
		t.Errorf("authority 331's same-name row must stay unread, got read_at %v", got)
	}
}

// TestIntegration_NotificationState_MarkApplicationsRead_Idempotent proves a second
// mark-read clears zero rows (idempotent) and does not bump the version a second
// time, while the first (cleared > 0) does bump it.
func TestIntegration_NotificationState_MarkApplicationsRead_Idempotent(t *testing.T) {
	s, pool := newPGStateStore(t)
	ctx := context.Background()
	now := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	t1 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	seedUnreadNotif(t, pool, "n1", "user-1", "24-01234", 330, t1)

	first, err := s.MarkApplicationsRead(ctx, "user-1", []string{"24-01234"}, []int{330}, now)
	if err != nil {
		t.Fatalf("first MarkApplicationsRead: %v", err)
	}
	if first != 1 {
		t.Errorf("first call cleared: got %d, want 1", first)
	}
	st, _ := s.Get(ctx, "user-1")
	if st == nil || st.Version != 1 {
		t.Fatalf("version after first clear: got %+v, want 1", st)
	}

	second, err := s.MarkApplicationsRead(ctx, "user-1", []string{"24-01234"}, []int{330}, now)
	if err != nil {
		t.Fatalf("second MarkApplicationsRead: %v", err)
	}
	if second != 0 {
		t.Errorf("second call cleared: got %d, want 0 (idempotent)", second)
	}
	// Zero cleared → no version bump.
	st, _ = s.Get(ctx, "user-1")
	if st == nil || st.Version != 1 {
		t.Errorf("version after zero-clear: got %+v, want unchanged 1", st)
	}
}

// TestIntegration_NotificationState_MarkApplicationsRead_EmptyClearsNothing proves an
// empty request clears nothing (never "all") and does not create or bump a state row.
func TestIntegration_NotificationState_MarkApplicationsRead_EmptyClearsNothing(t *testing.T) {
	s, pool := newPGStateStore(t)
	ctx := context.Background()
	now := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	t1 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	seedUnreadNotif(t, pool, "n1", "user-1", "24-01234", 330, t1)

	cleared, err := s.MarkApplicationsRead(ctx, "user-1", []string{}, []int{}, now)
	if err != nil {
		t.Fatalf("MarkApplicationsRead(empty): %v", err)
	}
	if cleared != 0 {
		t.Errorf("empty request cleared: got %d, want 0", cleared)
	}
	if got := unreadCountFor(t, pool, "user-1"); got != 1 {
		t.Errorf("unread after empty mark-read: got %d, want 1 (nothing cleared)", got)
	}
	if st, _ := s.Get(ctx, "user-1"); st != nil {
		t.Errorf("empty mark-read must not create a state row, got %+v", st)
	}
}

// readAtOf returns the (nullable) read_at for the notification with the given id.
func readAtOf(t *testing.T, pool *pgxpool.Pool, id string) *time.Time {
	t.Helper()
	var readAt *time.Time
	if err := pool.QueryRow(context.Background(),
		"SELECT read_at FROM notifications WHERE id = $1", id).Scan(&readAt); err != nil {
		t.Fatalf("scan read_at for %q: %v", id, err)
	}
	return readAt
}

// --- MarkReadUpTo (temporary advance compat shim, tc-ekii) ---

// TestIntegration_NotificationState_MarkReadUpTo proves the read_at translation of
// the retired watermark advance: notifications created at or before asOf are marked
// read (read_at NOT NULL), a later notification stays unread, a repeat clears zero
// (idempotent), the version token bumps only when a row was cleared, and a first-touch
// user gets a state row created on the first clear. This is real-DB behaviour (the
// created_at <= asOf predicate and the conditional CTE bump) the fake querier cannot
// model. Remove with the shim per bead tc-v5w8.
func TestIntegration_NotificationState_MarkReadUpTo(t *testing.T) {
	s, pool := newPGStateStore(t)
	ctx := context.Background()

	tm2 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC) // <= asOf
	tm1 := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC) // <= asOf
	asOf := time.Date(2026, 2, 15, 0, 0, 0, 0, time.UTC)
	tp1 := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC) // > asOf
	now := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)

	// All three start unread (Create leaves read_at NULL — the first-touch user has
	// no notification_state row yet).
	seedUnreadNotif(t, pool, "n1", "user-1", "24-01111", 330, tm2)
	seedUnreadNotif(t, pool, "n2", "user-1", "24-02222", 330, tm1)
	seedUnreadNotif(t, pool, "n3", "user-1", "24-03333", 330, tp1)
	if got := unreadCountFor(t, pool, "user-1"); got != 3 {
		t.Fatalf("seeded unread: got %d, want 3", got)
	}

	// Advance to asOf: the two at-or-before rows clear, the later one stays unread.
	cleared, err := s.MarkReadUpTo(ctx, "user-1", asOf, now)
	if err != nil {
		t.Fatalf("MarkReadUpTo: %v", err)
	}
	if cleared != 2 {
		t.Errorf("cleared: got %d, want 2 (rows created_at <= asOf)", cleared)
	}
	if got := readAtOf(t, pool, "n1"); got == nil {
		t.Error("n1 (created_at < asOf) must be read")
	}
	if got := readAtOf(t, pool, "n2"); got == nil {
		t.Error("n2 (created_at < asOf) must be read")
	}
	if got := readAtOf(t, pool, "n3"); got != nil {
		t.Errorf("n3 (created_at > asOf) must stay unread, got read_at %v", got)
	}

	// First-touch user: the clear upserts a state row at version 1.
	st, err := s.Get(ctx, "user-1")
	if err != nil {
		t.Fatalf("Get after advance: %v", err)
	}
	if st == nil || st.Version != 1 {
		t.Fatalf("state after first advance: got %+v, want version 1", st)
	}

	// A second advance to the same asOf clears nothing (idempotent) and does not
	// bump the version token again.
	second, err := s.MarkReadUpTo(ctx, "user-1", asOf, now)
	if err != nil {
		t.Fatalf("second MarkReadUpTo: %v", err)
	}
	if second != 0 {
		t.Errorf("second advance cleared: got %d, want 0 (idempotent)", second)
	}
	st, err = s.Get(ctx, "user-1")
	if err != nil {
		t.Fatalf("Get after second advance: %v", err)
	}
	if st == nil || st.Version != 1 {
		t.Errorf("version after zero-clear advance: got %+v, want unchanged 1", st)
	}
	if got := unreadCountFor(t, pool, "user-1"); got != 1 {
		t.Errorf("unread after advance: got %d, want 1 (only the post-asOf row)", got)
	}
}

// --- DeleteByUserID ---

func TestIntegration_NotificationState_DeleteByUserID(t *testing.T) {
	s, _ := newPGStateStore(t)
	ctx := context.Background()

	st := State{UserID: "user-1", LastReadAt: time.Now(), Version: 1}
	if err := s.Save(ctx, st); err != nil {
		t.Fatalf("Save: %v", err)
	}

	if err := s.DeleteByUserID(ctx, "user-1"); err != nil {
		t.Fatalf("DeleteByUserID: %v", err)
	}

	got, err := s.Get(ctx, "user-1")
	if err != nil {
		t.Fatalf("Get after delete: %v", err)
	}
	if got != nil {
		t.Errorf("expected nil after delete, got %+v", got)
	}
}

func TestIntegration_NotificationState_DeleteByUserID_MissingIsNotError(t *testing.T) {
	s, _ := newPGStateStore(t)
	ctx := context.Background()

	// Delete a user that has no state row — must not error.
	if err := s.DeleteByUserID(ctx, "nonexistent-user"); err != nil {
		t.Fatalf("DeleteByUserID on absent row: %v", err)
	}
}
