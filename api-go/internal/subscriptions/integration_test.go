//go:build integration

package subscriptions

import (
	"context"
	"testing"
	"time"

	"github.com/AmyDe/town-crier/api-go/internal/platform/postgres/pgtest"
)

// newNotifPGStore returns a Postgres-backed notification store over a
// truncated database. Integration tests must NOT call t.Parallel: pgtest.New
// holds a session-level advisory lock for the test's whole duration to
// serialise all packages that share the single docker-compose database.
func newNotifPGStore(t *testing.T) *PostgresNotificationStore {
	t.Helper()
	pool := pgtest.New(t)
	pgtest.Truncate(t, pool, "apple_notifications")
	return NewPostgresNotificationStore(pool, time.Now)
}

// TestPostgresNotificationStore_IsProcessed_RoundTrip marks a UUID and confirms
// IsProcessed returns true on the next call.
func TestPostgresNotificationStore_IsProcessed_RoundTrip(t *testing.T) {
	ctx := context.Background()
	store := newNotifPGStore(t)

	const uuid = "test-uuid-roundtrip"

	got, err := store.IsProcessed(ctx, uuid)
	if err != nil {
		t.Fatalf("IsProcessed before mark: %v", err)
	}
	if got {
		t.Fatal("want not processed before mark")
	}

	if err := store.MarkProcessed(ctx, uuid); err != nil {
		t.Fatalf("MarkProcessed: %v", err)
	}

	got, err = store.IsProcessed(ctx, uuid)
	if err != nil {
		t.Fatalf("IsProcessed after mark: %v", err)
	}
	if !got {
		t.Fatal("want processed after mark")
	}
}

// TestPostgresNotificationStore_MarkProcessed_Idempotent proves ON CONFLICT
// (notification_uuid) DO UPDATE absorbs a second mark of the same UUID without
// a unique-constraint error — matching CosmosNotificationStore's
// last-writer-wins UpsertItem behaviour.
func TestPostgresNotificationStore_MarkProcessed_Idempotent(t *testing.T) {
	ctx := context.Background()
	store := newNotifPGStore(t)

	const uuid = "test-uuid-idempotent"

	if err := store.MarkProcessed(ctx, uuid); err != nil {
		t.Fatalf("first MarkProcessed: %v", err)
	}
	if err := store.MarkProcessed(ctx, uuid); err != nil {
		t.Fatalf("second MarkProcessed (idempotent): %v", err)
	}

	// Exactly one row in the table.
	var count int
	if err := store.db.QueryRow(ctx,
		"SELECT count(*) FROM apple_notifications WHERE notification_uuid = $1", uuid,
	).Scan(&count); err != nil {
		t.Fatalf("count rows: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 row after two marks, got %d", count)
	}
}
