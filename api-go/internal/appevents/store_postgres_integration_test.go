//go:build integration

package appevents

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/AmyDe/town-crier/api-go/internal/platform/postgres/pgtest"
)

func newPGStore(t *testing.T) (*PostgresStore, *pgxpool.Pool) {
	t.Helper()
	pool := pgtest.New(t)
	pgtest.Truncate(t, pool, "application_event")
	return NewPostgresStore(pool), pool
}

func insertEvent(t *testing.T, pool *pgxpool.Pool, uid, kind string, eventDate *time.Time, detectedAt time.Time) int64 {
	t.Helper()
	var id int64
	err := pool.QueryRow(context.Background(),
		`INSERT INTO application_event (uid, authority_code, kind, event_date, detected_at)
		 VALUES ($1, '1', $2, $3, $4) RETURNING id`, uid, kind, eventDate, detectedAt).Scan(&id)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func statusOf(t *testing.T, pool *pgxpool.Pool, id int64) (string, bool) {
	t.Helper()
	var status string
	var processed *time.Time
	if err := pool.QueryRow(context.Background(), "SELECT status, processed_at FROM application_event WHERE id = $1", id).Scan(&status, &processed); err != nil {
		t.Fatal(err)
	}
	return status, processed != nil
}

func TestPostgresStore_PendingOldestFirst(t *testing.T) {
	store, pool := newPGStore(t)
	ctx := context.Background()
	now := time.Now()
	late := insertEvent(t, pool, "late", KindNewApplication, nil, now.Add(-time.Hour))
	early := insertEvent(t, pool, "early", KindDecision, date(2026, 6, 1), now.Add(-3*time.Hour))
	mid := insertEvent(t, pool, "mid", KindNewApplication, nil, now.Add(-2*time.Hour))
	sent := insertEvent(t, pool, "sent", KindNewApplication, nil, now.Add(-4*time.Hour))
	if err := store.MarkSent(ctx, sent, now); err != nil {
		t.Fatal(err)
	}

	got, err := store.Pending(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 || got[0].ID != early || got[1].ID != mid || got[2].ID != late {
		t.Fatalf("pending = %+v", got)
	}
	if got[0].Kind != KindDecision || got[0].UID != "early" || got[0].AuthorityCode != "1" ||
		got[0].EventDate == nil || got[0].EventDate.Format("2006-01-02") != "2026-06-01" || got[1].EventDate != nil {
		t.Errorf("fields = %+v", got[:2])
	}
}

func TestPostgresStore_StatusTransitions(t *testing.T) {
	store, pool := newPGStore(t)
	ctx := context.Background()
	now := time.Now()
	a := insertEvent(t, pool, "a", KindNewApplication, nil, now)
	b := insertEvent(t, pool, "b", KindNewApplication, nil, now)
	c := insertEvent(t, pool, "c", KindNewApplication, nil, now)

	if err := store.MarkSent(ctx, a, now); err != nil {
		t.Fatal(err)
	}
	if err := store.MarkStale(ctx, []int64{b}, now); err != nil {
		t.Fatal(err)
	}
	for id, want := range map[int64]string{a: StatusSent, b: StatusStale, c: StatusPending} {
		st, processed := statusOf(t, pool, id)
		if st != want || processed != (want != StatusPending) {
			t.Errorf("event %d: status = %s processed = %v, want %s", id, st, processed, want)
		}
	}
}

func TestPostgresStore_CountActiveSince(t *testing.T) {
	store, pool := newPGStore(t)
	ctx := context.Background()
	now := time.Now()
	since := now.Add(-24 * time.Hour)
	insertEvent(t, pool, "old", KindNewApplication, nil, now.Add(-25*time.Hour))
	insertEvent(t, pool, "pending", KindNewApplication, nil, now.Add(-time.Hour))
	sent := insertEvent(t, pool, "sent", KindNewApplication, nil, now.Add(-2*time.Hour))
	stale := insertEvent(t, pool, "stale", KindNewApplication, nil, now.Add(-3*time.Hour))
	if err := store.MarkSent(ctx, sent, now); err != nil {
		t.Fatal(err)
	}
	if err := store.MarkStale(ctx, []int64{stale}, now); err != nil {
		t.Fatal(err)
	}

	n, err := store.CountActiveSince(ctx, since)
	if err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Errorf("count = %d, want 2 (pending + sent inside window)", n)
	}
}
