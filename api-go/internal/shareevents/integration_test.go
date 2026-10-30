//go:build integration

package shareevents

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/AmyDe/town-crier/api-go/internal/platform/postgres/pgtest"
)

func TestPostgresStore_IncrementUpsertsDailyTotals(t *testing.T) {
	pool := pgtest.New(t)
	pgtest.Truncate(t, pool, "share_event_totals")
	store := NewPostgresStore(pool)
	ctx := context.Background()
	day := time.Date(2026, 10, 30, 0, 0, 0, 0, time.UTC)

	var wg sync.WaitGroup
	for range 5 {
		wg.Go(func() {
			if err := store.Increment(ctx, day, "mail"); err != nil {
				t.Errorf("Increment: %v", err)
			}
		})
	}
	wg.Wait()
	if err := store.Increment(ctx, day, "copy"); err != nil {
		t.Fatalf("Increment: %v", err)
	}
	if err := store.Increment(ctx, day.AddDate(0, 0, 1), "mail"); err != nil {
		t.Fatalf("Increment: %v", err)
	}

	for _, tc := range []struct {
		day         time.Time
		destination string
		want        int64
	}{
		{day, "mail", 5},
		{day, "copy", 1},
		{day.AddDate(0, 0, 1), "mail", 1},
	} {
		var got int64
		err := pool.QueryRow(ctx,
			`SELECT total FROM share_event_totals WHERE day = $1 AND destination = $2`,
			tc.day, tc.destination).Scan(&got)
		if err != nil {
			t.Fatalf("read %s: %v", tc.destination, err)
		}
		if got != tc.want {
			t.Errorf("%s on %s = %d, want %d", tc.destination, tc.day, got, tc.want)
		}
	}
}
