//go:build integration

package notifications

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/AmyDe/town-crier/api-go/internal/platform/postgres/pgtest"
	"github.com/jackc/pgx/v5/pgxpool"
)

// These statements mirror migrations/0015_notifications_read_at.sql exactly. The
// goose migration runs once, against the empty test database, when pgtest.New
// applies the schema — so it cannot observe fixtures seeded afterwards. To
// exercise the backfill against real rows we replay the identical UPDATEs here
// after seeding a pre-migration state (notifications with read_at IS NULL plus a
// watermark row), then assert the resulting read_at values. Keep these in sync
// with the migration.
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

// TestIntegration_Migration0015_ReadAtBackfill asserts the acceptance criteria for
// the read_at backfill: a notification at or before a watermarked user's
// last_read_at gets a non-null read_at (== the watermark); one after the watermark
// stays NULL; and a notification for a user with no watermark row gets read_at ==
// created_at.
func TestIntegration_Migration0015_ReadAtBackfill(t *testing.T) {
	pool := pgtest.New(t)
	pgtest.Truncate(t, pool, "notifications", "notification_state")
	store := NewPostgresStore(pool)
	ctx := context.Background()

	watermark := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	before := watermark.Add(-24 * time.Hour) // strictly before the watermark
	after := watermark.Add(24 * time.Hour)   // strictly after the watermark
	nowmCreated := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)

	// Seed via the store's Create (which does not set read_at, so every seeded row
	// starts read_at IS NULL — the pre-migration state).
	for _, n := range []DigestNotification{
		intNotif("pre", "u-wm", "uid-pre", EventNewApplication, before),
		intNotif("exact", "u-wm", "uid-exact", EventNewApplication, watermark),
		intNotif("post", "u-wm", "uid-post", EventNewApplication, after),
		intNotif("nowm", "u-nowm", "uid-nowm", EventNewApplication, nowmCreated),
	} {
		if err := store.Create(ctx, n); err != nil {
			t.Fatalf("seed %s: %v", n.ID, err)
		}
	}

	// Watermark row for u-wm only; u-nowm has none.
	if _, err := pool.Exec(ctx,
		"INSERT INTO notification_state (user_id, last_read_at, version) VALUES ($1, $2, $3)",
		"u-wm", watermark, 1); err != nil {
		t.Fatalf("seed watermark: %v", err)
	}

	// Every row must start unread (read_at NULL) before the backfill runs.
	for _, id := range []string{"pre", "exact", "post", "nowm"} {
		if got := readReadAt(t, pool, id); got != nil {
			t.Fatalf("pre-backfill read_at for %s: got %v, want NULL", id, got)
		}
	}

	// Replay the migration backfill.
	if _, err := pool.Exec(ctx, backfillWatermarkedSQL); err != nil {
		t.Fatalf("backfill watermarked: %v", err)
	}
	if _, err := pool.Exec(ctx, backfillNoWatermarkSQL); err != nil {
		t.Fatalf("backfill no-watermark: %v", err)
	}

	// At/before the watermark → read_at == watermark (inclusive boundary).
	assertReadAt(t, pool, "pre", &watermark)
	assertReadAt(t, pool, "exact", &watermark)
	// After the watermark → still unread.
	assertReadAt(t, pool, "post", nil)
	// No watermark row → read_at == the notification's own created_at.
	assertReadAt(t, pool, "nowm", &nowmCreated)
}

// TestIntegration_Migration0015_PartialIndexExists proves migration 0015's
// CREATE INDEX ran: the partial index over the unread set exists with the expected
// columns and read_at IS NULL predicate.
func TestIntegration_Migration0015_PartialIndexExists(t *testing.T) {
	pool := pgtest.New(t)

	var indexDef string
	err := pool.QueryRow(context.Background(),
		"SELECT indexdef FROM pg_indexes WHERE indexname = 'idx_notifications_unread'").
		Scan(&indexDef)
	if err != nil {
		t.Fatalf("query idx_notifications_unread def: %v", err)
	}

	for _, want := range []string{"user_id", "application_uid", "created_at", "read_at IS NULL"} {
		if !strings.Contains(indexDef, want) {
			t.Errorf("index def %q missing %q", indexDef, want)
		}
	}
}

// readReadAt returns the (nullable) read_at for the notification with the given id.
func readReadAt(t *testing.T, pool *pgxpool.Pool, id string) *time.Time {
	t.Helper()
	var readAt *time.Time
	if err := pool.QueryRow(context.Background(),
		"SELECT read_at FROM notifications WHERE id = $1", id).Scan(&readAt); err != nil {
		t.Fatalf("scan read_at for %s: %v", id, err)
	}
	return readAt
}

// assertReadAt asserts the notification's read_at matches want (nil = NULL).
func assertReadAt(t *testing.T, pool *pgxpool.Pool, id string, want *time.Time) {
	t.Helper()
	got := readReadAt(t, pool, id)
	switch {
	case want == nil && got != nil:
		t.Errorf("%s read_at: got %v, want NULL", id, got)
	case want != nil && got == nil:
		t.Errorf("%s read_at: got NULL, want %v", id, *want)
	case want != nil && got != nil && !got.Equal(*want):
		t.Errorf("%s read_at: got %v, want %v", id, *got, *want)
	}
}
