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

// --- UnreadCount cross-reads the notifications table ---

func TestIntegration_NotificationState_UnreadCount(t *testing.T) {
	s, pool := newPGStateStore(t)
	ctx := context.Background()

	// Seed two notifications for user-1 using the notifications PostgresStore
	// (same pool — correct cross-table read).
	nStore := notifications.NewPostgresStore(pool)
	t1 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	t2 := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)
	t3 := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)

	for _, n := range []notifications.DigestNotification{
		{ID: "n1", UserID: "user-1", ApplicationUID: "uid-A", ApplicationName: "A",
			AuthorityID: 100, EventType: notifications.EventNewApplication,
			Sources: "Zone", CreatedAt: t1},
		{ID: "n2", UserID: "user-1", ApplicationUID: "uid-B", ApplicationName: "B",
			AuthorityID: 100, EventType: notifications.EventNewApplication,
			Sources: "Zone", CreatedAt: t2},
		{ID: "n3", UserID: "user-1", ApplicationUID: "uid-C", ApplicationName: "C",
			AuthorityID: 100, EventType: notifications.EventNewApplication,
			Sources: "Zone", CreatedAt: t3},
	} {
		if err := nStore.Create(ctx, n); err != nil {
			t.Fatalf("seed notification %s: %v", n.ID, err)
		}
	}

	// lastReadAt = t1 → n2 (t2) and n3 (t3) are unread.
	count, err := s.UnreadCount(ctx, "user-1", t1)
	if err != nil {
		t.Fatalf("UnreadCount: %v", err)
	}
	if count != 2 {
		t.Errorf("unread count: got %d, want 2", count)
	}

	// lastReadAt = t3 (at the boundary) → 0 unread (strictly after, t3 itself is read).
	countAtBoundary, err := s.UnreadCount(ctx, "user-1", t3)
	if err != nil {
		t.Fatalf("UnreadCount at boundary: %v", err)
	}
	if countAtBoundary != 0 {
		t.Errorf("unread count at boundary: got %d, want 0", countAtBoundary)
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
