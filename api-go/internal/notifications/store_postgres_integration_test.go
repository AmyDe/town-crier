//go:build integration

package notifications

import (
	"context"
	"testing"
	"time"

	"github.com/AmyDe/town-crier/api-go/internal/platform/postgres/pgtest"
)

// newPGNotifStore returns a notifications PostgresStore over a freshly
// truncated integration-test database. Integration tests must NOT call
// t.Parallel: pgtest.New holds a session-level advisory lock ensuring all
// integration tests run serially across the whole module.
func newPGNotifStore(t *testing.T) *PostgresStore {
	t.Helper()
	pool := pgtest.New(t)
	pgtest.Truncate(t, pool, "notifications", "notification_state")
	return NewPostgresStore(pool)
}

// fixtures

func intNotif(id, userID, applicationUID string, eventType EventType, createdAt time.Time) DigestNotification {
	return DigestNotification{
		ID:                     id,
		UserID:                 userID,
		ApplicationUID:         applicationUID,
		ApplicationName:        "22/TEST",
		ApplicationAddress:     "1 Test Street",
		ApplicationDescription: "outline planning",
		AuthorityID:            100,
		EventType:              eventType,
		Sources:                "Zone",
		CreatedAt:              createdAt,
	}
}

// --- Create / GetByUserAndApplication (dedup) ---

func TestIntegration_Notifications_CreateAndGetByUserAndApplication(t *testing.T) {
	s := newPGNotifStore(t)
	ctx := context.Background()
	n := intNotif("id-1", "user-1", "uid-A", EventNewApplication,
		time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC))

	if err := s.Create(ctx, n); err != nil {
		t.Fatalf("Create: %v", err)
	}

	got, err := s.GetByUserAndApplication(ctx, "user-1", "uid-A", 100, EventNewApplication)
	if err != nil {
		t.Fatalf("GetByUserAndApplication: %v", err)
	}
	if got == nil {
		t.Fatal("expected notification, got nil")
	}
	if got.ID != "id-1" {
		t.Errorf("ID: got %q, want id-1", got.ID)
	}
	if got.EventType != EventNewApplication {
		t.Errorf("EventType: got %q, want NewApplication", got.EventType)
	}
}

func TestIntegration_Notifications_Create_Idempotent(t *testing.T) {
	s := newPGNotifStore(t)
	ctx := context.Background()
	n := intNotif("id-1", "user-1", "uid-A", EventNewApplication,
		time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC))

	if err := s.Create(ctx, n); err != nil {
		t.Fatalf("first Create: %v", err)
	}
	// Same id — ON CONFLICT DO UPDATE must not error.
	if err := s.Create(ctx, n); err != nil {
		t.Fatalf("second Create (idempotent): %v", err)
	}

	// Only one row should exist.
	all, err := s.AllByUser(ctx, "user-1")
	if err != nil {
		t.Fatalf("AllByUser: %v", err)
	}
	if len(all) != 1 {
		t.Errorf("row count after idempotent create: got %d, want 1", len(all))
	}
}

func TestIntegration_Notifications_GetByUserAndApplication_NotFound(t *testing.T) {
	s := newPGNotifStore(t)
	ctx := context.Background()

	got, err := s.GetByUserAndApplication(ctx, "user-x", "uid-x", 100, EventNewApplication)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != nil {
		t.Errorf("expected nil for absent notification, got %+v", got)
	}
}

// --- GetLatestUnreadByApplications ---

func TestIntegration_Notifications_GetLatestUnreadByApplications(t *testing.T) {
	s := newPGNotifStore(t)
	ctx := context.Background()

	t1 := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)
	t2 := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)

	// uid-A: two notifications; the newer (t2) must win.
	if err := s.Create(ctx, intNotif("n1", "user-1", "uid-A", EventNewApplication, t1)); err != nil {
		t.Fatalf("Create n1: %v", err)
	}
	if err := s.Create(ctx, intNotif("n2", "user-1", "uid-A", EventDecisionUpdate, t2)); err != nil {
		t.Fatalf("Create n2: %v", err)
	}
	// uid-B: one notification.
	if err := s.Create(ctx, intNotif("n3", "user-1", "uid-B", EventNewApplication, t1)); err != nil {
		t.Fatalf("Create n3: %v", err)
	}

	// All three rows are unread (read_at IS NULL — Create never sets read_at).
	got, err := s.GetLatestUnreadByApplications(ctx, "user-1", []string{"uid-A", "uid-B"})
	if err != nil {
		t.Fatalf("GetLatestUnreadByApplications: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("map size: got %d, want 2", len(got))
	}
	a := got["uid-A"]
	if a.EventType != EventDecisionUpdate {
		t.Errorf("uid-A event: got %q, want DecisionUpdate", a.EventType)
	}
	if !a.CreatedAt.Equal(t2) {
		t.Errorf("uid-A createdAt: got %v, want %v", a.CreatedAt, t2)
	}
}

// TestIntegration_Notifications_GetLatestUnreadByApplications_ExcludesRead proves a
// notification with read_at set (read) is excluded — the read_at IS NULL predicate
// (ADR 0035). This matches what an equivalent watermark past the notification's
// created_at would have excluded.
func TestIntegration_Notifications_GetLatestUnreadByApplications_ExcludesRead(t *testing.T) {
	pool := pgtest.New(t)
	pgtest.Truncate(t, pool, "notifications", "notification_state")
	s := NewPostgresStore(pool)
	ctx := context.Background()

	t1 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	t2 := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)

	// Notification at t1, then marked read (read_at = t2) — the equivalent of a
	// watermark past t1.
	if err := s.Create(ctx, intNotif("n1", "user-1", "uid-A", EventNewApplication, t1)); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if _, err := pool.Exec(ctx, "UPDATE notifications SET read_at = $1 WHERE id = 'n1'", t2); err != nil {
		t.Fatalf("mark n1 read: %v", err)
	}

	got, err := s.GetLatestUnreadByApplications(ctx, "user-1", []string{"uid-A"})
	if err != nil {
		t.Fatalf("GetLatestUnreadByApplications: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected empty map (all read), got %d entries", len(got))
	}
}

// --- ByUserSince / AllByUser ---

func TestIntegration_Notifications_ByUserSince(t *testing.T) {
	s := newPGNotifStore(t)
	ctx := context.Background()

	t1 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	t2 := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)
	t3 := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)

	for _, n := range []DigestNotification{
		intNotif("n1", "user-1", "uid-A", EventNewApplication, t1),
		intNotif("n2", "user-1", "uid-B", EventNewApplication, t2),
		intNotif("n3", "user-1", "uid-C", EventNewApplication, t3),
	} {
		if err := s.Create(ctx, n); err != nil {
			t.Fatalf("Create %s: %v", n.ID, err)
		}
	}

	// since = t2 (inclusive) → expect n2 and n3 only.
	got, err := s.ByUserSince(ctx, "user-1", t2)
	if err != nil {
		t.Fatalf("ByUserSince: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("count: got %d, want 2", len(got))
	}
}

func TestIntegration_Notifications_AllByUser(t *testing.T) {
	s := newPGNotifStore(t)
	ctx := context.Background()

	t1 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	t2 := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)

	if err := s.Create(ctx, intNotif("n1", "user-1", "uid-A", EventNewApplication, t1)); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := s.Create(ctx, intNotif("n2", "user-1", "uid-B", EventNewApplication, t2)); err != nil {
		t.Fatalf("Create: %v", err)
	}
	// Different user — must not be returned.
	if err := s.Create(ctx, intNotif("n3", "user-2", "uid-C", EventNewApplication, t1)); err != nil {
		t.Fatalf("Create user-2: %v", err)
	}

	got, err := s.AllByUser(ctx, "user-1")
	if err != nil {
		t.Fatalf("AllByUser: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("count: got %d, want 2 (user-1 only)", len(got))
	}
	// AllByUser returns oldest first.
	if got[0].ID != "n1" {
		t.Errorf("first row ID: got %q, want n1 (oldest first)", got[0].ID)
	}
}

// --- UnsentEmailsByUser / UserIDsWithUnsentEmails / MarkEmailSent ---

func TestIntegration_Notifications_UnsentEmailsFlow(t *testing.T) {
	s := newPGNotifStore(t)
	ctx := context.Background()

	t1 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	n := intNotif("n1", "user-1", "uid-A", EventNewApplication, t1)
	if err := s.Create(ctx, n); err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Before marking: should appear in unsent list.
	unsent, err := s.UnsentEmailsByUser(ctx, "user-1")
	if err != nil {
		t.Fatalf("UnsentEmailsByUser before mark: %v", err)
	}
	if len(unsent) != 1 {
		t.Fatalf("unsent count before mark: got %d, want 1", len(unsent))
	}

	// MarkEmailSent.
	if err := s.MarkEmailSent(ctx, unsent[0]); err != nil {
		t.Fatalf("MarkEmailSent: %v", err)
	}

	// After marking: should disappear from unsent list.
	after, err := s.UnsentEmailsByUser(ctx, "user-1")
	if err != nil {
		t.Fatalf("UnsentEmailsByUser after mark: %v", err)
	}
	if len(after) != 0 {
		t.Errorf("unsent count after mark: got %d, want 0", len(after))
	}
}

func TestIntegration_Notifications_UserIDsWithUnsentEmails(t *testing.T) {
	s := newPGNotifStore(t)
	ctx := context.Background()

	t1 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	if err := s.Create(ctx, intNotif("n1", "user-A", "uid-1", EventNewApplication, t1)); err != nil {
		t.Fatalf("Create user-A: %v", err)
	}
	if err := s.Create(ctx, intNotif("n2", "user-B", "uid-2", EventNewApplication, t1)); err != nil {
		t.Fatalf("Create user-B: %v", err)
	}
	// Two notifications for user-A — should appear only once.
	if err := s.Create(ctx, intNotif("n3", "user-A", "uid-3", EventDecisionUpdate, t1)); err != nil {
		t.Fatalf("Create user-A n3: %v", err)
	}

	ids, err := s.UserIDsWithUnsentEmails(ctx)
	if err != nil {
		t.Fatalf("UserIDsWithUnsentEmails: %v", err)
	}
	if len(ids) != 2 {
		t.Errorf("unique user count: got %d, want 2 (user-A and user-B)", len(ids))
	}
}

// --- DeleteAllByUserID ---

func TestIntegration_Notifications_DeleteAllByUserID(t *testing.T) {
	s := newPGNotifStore(t)
	ctx := context.Background()

	t1 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	if err := s.Create(ctx, intNotif("n1", "user-1", "uid-A", EventNewApplication, t1)); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := s.Create(ctx, intNotif("n2", "user-1", "uid-B", EventNewApplication, t1)); err != nil {
		t.Fatalf("Create: %v", err)
	}
	// Different user — must survive the delete.
	if err := s.Create(ctx, intNotif("n3", "user-2", "uid-C", EventNewApplication, t1)); err != nil {
		t.Fatalf("Create user-2: %v", err)
	}

	if err := s.DeleteAllByUserID(ctx, "user-1"); err != nil {
		t.Fatalf("DeleteAllByUserID: %v", err)
	}

	remaining, err := s.AllByUser(ctx, "user-1")
	if err != nil {
		t.Fatalf("AllByUser after delete: %v", err)
	}
	if len(remaining) != 0 {
		t.Errorf("user-1 notifications after delete: got %d, want 0", len(remaining))
	}

	// user-2's row must be intact.
	other, err := s.AllByUser(ctx, "user-2")
	if err != nil {
		t.Fatalf("AllByUser user-2: %v", err)
	}
	if len(other) != 1 {
		t.Errorf("user-2 notifications after user-1 delete: got %d, want 1", len(other))
	}
}

// --- PurgeOlderThan ---

func TestIntegration_Notifications_PurgeOlderThan(t *testing.T) {
	s := newPGNotifStore(t)
	ctx := context.Background()

	old := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	recent := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	cutoff := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)

	if err := s.Create(ctx, intNotif("n-old", "user-1", "uid-A", EventNewApplication, old)); err != nil {
		t.Fatalf("Create old: %v", err)
	}
	if err := s.Create(ctx, intNotif("n-recent", "user-1", "uid-B", EventNewApplication, recent)); err != nil {
		t.Fatalf("Create recent: %v", err)
	}

	deleted, err := s.PurgeOlderThan(ctx, cutoff)
	if err != nil {
		t.Fatalf("PurgeOlderThan: %v", err)
	}
	if deleted != 1 {
		t.Errorf("rows deleted: got %d, want 1 (only the old row)", deleted)
	}

	// Recent row must survive.
	all, err := s.AllByUser(ctx, "user-1")
	if err != nil {
		t.Fatalf("AllByUser after purge: %v", err)
	}
	if len(all) != 1 || all[0].ID != "n-recent" {
		t.Errorf("post-purge rows: got %+v, want only n-recent", all)
	}
}
