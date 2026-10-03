//go:build integration

package polling

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/AmyDe/town-crier/api-go/internal/planit"
	"github.com/AmyDe/town-crier/api-go/internal/platform/postgres/pgtest"
)

func newPGPacer(t *testing.T, now time.Time, cap int) (*Pacer, *PostgresPlanItCallStore, *fakeClock) {
	t.Helper()
	pool := pgtest.New(t)
	pgtest.Truncate(t, pool, "planit_call")
	store := NewPostgresPlanItCallStore(pool)
	clk := &fakeClock{now: now}
	return NewPacer(store, PacerConfig{DailyCap: cap, MinSpacing: 60 * time.Second}, clk.Now, clk.Sleep), store, clk
}

func TestPostgresPlanItCallStore_DoRoundTripsRowFields(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 6, 10, 20, 0, 0, 0, londonTZ)
	p, store, _ := newPGPacer(t, now, 300)
	day := time.Date(2026, 6, 9, 0, 0, 0, 0, time.UTC)

	_, err := p.Do(ctx, planit.WorkWindowDecided, day, 3, func(context.Context) (planit.FetchPageResult, error) {
		return planit.FetchPageResult{}, &planit.RateLimitError{RetryAfter: d(90 * time.Second)}
	})
	var rl *planit.RateLimitError
	if !errors.As(err, &rl) {
		t.Fatalf("err = %v", err)
	}
	got, err := store.Latest(ctx)
	if err != nil || got == nil {
		t.Fatalf("Latest = %v, %v", got, err)
	}
	if got.Work != "window_decided" || got.PageIndex != 3 || got.WindowDay == nil || !got.WindowDay.Equal(day) {
		t.Fatalf("row = %+v", got)
	}
	if got.Status == nil || *got.Status != 429 || got.RetryAfter == nil || *got.RetryAfter != 90*time.Second {
		t.Fatalf("result = status %v retry %v", got.Status, got.RetryAfter)
	}
	if !got.At.Equal(now) {
		t.Fatalf("at = %s, want %s", got.At, now)
	}

	st, err := p.Backoff(ctx, now)
	if err != nil || !st.Active || !st.Until.Equal(now.Add(90*time.Second)) {
		t.Fatalf("Backoff = %+v, %v", st, err)
	}
	if _, err := p.Do(ctx, planit.WorkWindowStart, time.Time{}, 0, okWork(1)); !errors.Is(err, ErrBackedOff) {
		t.Fatalf("err = %v, want ErrBackedOff", err)
	}
	n, err := p.CallsToday(ctx, now)
	if err != nil || n != 1 {
		t.Fatalf("CallsToday = %d, %v; want 1 (backed-off call writes no row)", n, err)
	}
}

func TestPostgresPlanItCallStore_NullWindowDayAndTimeout(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 6, 10, 20, 0, 0, 0, londonTZ)
	p, store, _ := newPGPacer(t, now, 300)

	_, _ = p.Do(ctx, planit.WorkDeltaStart, time.Time{}, 0, func(context.Context) (planit.FetchPageResult, error) {
		return planit.FetchPageResult{}, planitTimeout()
	})
	got, err := store.Latest(ctx)
	if err != nil || got == nil {
		t.Fatalf("Latest = %v, %v", got, err)
	}
	if got.WindowDay != nil || got.Status != nil || got.Total != nil || got.RetryAfter != nil {
		t.Fatalf("row = %+v, want null window_day/status/total/retry_after", got)
	}
}

func TestPostgresPlanItCallStore_CapCountsRealRows(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 6, 10, 20, 0, 0, 0, londonTZ)
	p, store, _ := newPGPacer(t, now, 3)

	for i := 0; i < 3; i++ {
		if _, err := p.Do(ctx, planit.WorkWindowStart, time.Time{}, i, okWork(10)); err != nil {
			t.Fatalf("Do %d: %v", i, err)
		}
	}
	if _, err := p.Do(ctx, planit.WorkWindowStart, time.Time{}, 3, okWork(10)); !errors.Is(err, ErrBudgetExhausted) {
		t.Fatalf("err = %v, want ErrBudgetExhausted", err)
	}
	n, err := store.CountBetween(ctx, now.Add(-time.Hour), now.Add(time.Hour))
	if err != nil || n != 3 {
		t.Fatalf("CountBetween = %d, %v; want 3", n, err)
	}
}

func TestPostgresPlanItCallStore_BudgetDayBoundaryAcrossDST(t *testing.T) {
	ctx := context.Background()
	// 2026-03-28 18:00 GMT to 2026-03-29 18:00 BST is a 23h budget day.
	now := time.Date(2026, 3, 29, 17, 30, 0, 0, londonTZ)
	p, _, _ := newPGPacer(t, now, 1)
	if _, err := p.Do(ctx, planit.WorkWindowStart, time.Time{}, 0, okWork(1)); err != nil {
		t.Fatalf("first: %v", err)
	}
	if _, err := p.Do(ctx, planit.WorkWindowStart, time.Time{}, 1, okWork(1)); !errors.Is(err, ErrBudgetExhausted) {
		t.Fatalf("second err = %v, want ErrBudgetExhausted", err)
	}
}

func TestPacer_Postgres_ConcurrentCallersHoldSpacingAndCap(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 6, 10, 20, 0, 0, 0, londonTZ)
	p, store, _ := newPGPacer(t, now, 4)

	var wg sync.WaitGroup
	var mu sync.Mutex
	sent, exhausted := 0, 0
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := p.Do(ctx, planit.WorkWindowStart, time.Time{}, 0, okWork(1))
			mu.Lock()
			defer mu.Unlock()
			switch {
			case err == nil:
				sent++
			case errors.Is(err, ErrBudgetExhausted):
				exhausted++
			default:
				t.Errorf("unexpected error: %v", err)
			}
		}()
	}
	wg.Wait()
	if sent != 4 || exhausted != 4 {
		t.Fatalf("sent=%d exhausted=%d, want 4 and 4", sent, exhausted)
	}
	rows, err := store.pool.Query(ctx, "SELECT at FROM planit_call ORDER BY at")
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var prev time.Time
	n := 0
	for rows.Next() {
		var at time.Time
		if err := rows.Scan(&at); err != nil {
			t.Fatal(err)
		}
		if n > 0 && at.Sub(prev) < 60*time.Second {
			t.Fatalf("gap %d = %s, want >= 60s", n, at.Sub(prev))
		}
		prev = at
		n++
	}
	if n != 4 {
		t.Fatalf("rows = %d, want 4", n)
	}
}
