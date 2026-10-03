//go:build integration

package polling

import (
	"context"
	"testing"
	"time"

	"github.com/AmyDe/town-crier/api-go/internal/planit"
	"github.com/AmyDe/town-crier/api-go/internal/platform/postgres/pgtest"
)

func newWindowStore(t *testing.T) (*PostgresWindowStore, *PostgresPlanItCallStore) {
	t.Helper()
	pool := pgtest.New(t)
	pgtest.Truncate(t, pool, "poll_window", "planit_call")
	return NewPostgresWindowStore(pool), NewPostgresPlanItCallStore(pool)
}

func TestPostgresWindowStore_MarkFullReadThenLoad(t *testing.T) {
	ctx := context.Background()
	s, _ := newWindowStore(t)
	ref := WindowRef{Axis: planit.AxisDecided, Day: utcDay(2026, 6, 9)}
	at := time.Date(2026, 6, 10, 21, 0, 0, 0, time.UTC)

	if err := s.MarkFullRead(ctx, ref, at, 1800); err != nil {
		t.Fatal(err)
	}
	got, err := s.Load(ctx, utcDay(2026, 5, 1), utcDay(2026, 6, 10))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d rows", len(got))
	}
	w := got[0]
	if w.Axis != planit.AxisDecided || !w.Day.Equal(ref.Day) || w.LastCompleteAt == nil || !w.LastCompleteAt.Equal(at) ||
		w.LastFullReadAt == nil || !w.LastFullReadAt.Equal(at) || w.LastTotal == nil || *w.LastTotal != 1800 {
		t.Fatalf("row = %+v", w)
	}
}

func TestPostgresWindowStore_ProbeKeepsFullReadAndTotal(t *testing.T) {
	ctx := context.Background()
	s, _ := newWindowStore(t)
	ref := WindowRef{Axis: planit.AxisStart, Day: utcDay(2026, 6, 9)}
	full := time.Date(2026, 6, 8, 21, 0, 0, 0, time.UTC)
	probe := time.Date(2026, 6, 9, 21, 0, 0, 0, time.UTC)

	if err := s.MarkFullRead(ctx, ref, full, 500); err != nil {
		t.Fatal(err)
	}
	if err := s.MarkProbeComplete(ctx, ref, probe); err != nil {
		t.Fatal(err)
	}
	got, _ := s.Load(ctx, ref.Day, ref.Day)
	w := got[0]
	if !w.LastCompleteAt.Equal(probe) || !w.LastFullReadAt.Equal(full) || *w.LastTotal != 500 {
		t.Fatalf("row = %+v", w)
	}
}

func TestPostgresWindowStore_ProbeOnUnknownWindowCreatesRow(t *testing.T) {
	ctx := context.Background()
	s, _ := newWindowStore(t)
	ref := WindowRef{Axis: planit.AxisStart, Day: utcDay(2026, 6, 9)}

	if err := s.MarkProbeComplete(ctx, ref, time.Now()); err != nil {
		t.Fatal(err)
	}
	got, _ := s.Load(ctx, ref.Day, ref.Day)
	if len(got) != 1 || got[0].LastFullReadAt != nil || got[0].LastTotal != nil {
		t.Fatalf("got %+v", got)
	}
}

func TestPostgresWindowStore_LoadFiltersByDayRange(t *testing.T) {
	ctx := context.Background()
	s, _ := newWindowStore(t)
	at := time.Now()
	for _, d := range []int{1, 5, 9} {
		if err := s.MarkFullRead(ctx, WindowRef{Axis: planit.AxisStart, Day: utcDay(2026, 6, d)}, at, 1); err != nil {
			t.Fatal(err)
		}
	}
	got, err := s.Load(ctx, utcDay(2026, 6, 2), utcDay(2026, 6, 9))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d rows, want 2 (inclusive range)", len(got))
	}
}

func TestPostgresWindowStore_LatestPageZeroAt(t *testing.T) {
	ctx := context.Background()
	s, calls := newWindowStore(t)

	got, err := s.LatestPageZeroAt(ctx, planit.WorkDeltaStart)
	if err != nil || got != nil {
		t.Fatalf("empty log: %v %v", got, err)
	}

	insert := func(work planit.Work, page int, at time.Time) {
		t.Helper()
		tx, err := calls.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := tx.Insert(ctx, PlanItCall{At: at, Work: string(work), PageIndex: page}); err != nil {
			t.Fatal(err)
		}
		if err := tx.Commit(ctx); err != nil {
			t.Fatal(err)
		}
	}
	base := time.Date(2026, 6, 10, 9, 0, 0, 0, time.UTC)
	insert(planit.WorkDeltaStart, 0, base)
	insert(planit.WorkDeltaStart, 1, base.Add(2*time.Hour))
	insert(planit.WorkDeltaDecided, 0, base.Add(3*time.Hour))
	insert(planit.WorkDeltaStart, 0, base.Add(time.Hour))

	got, err = s.LatestPageZeroAt(ctx, planit.WorkDeltaStart)
	if err != nil || got == nil || !got.Equal(base.Add(time.Hour)) {
		t.Fatalf("got %v, %v", got, err)
	}
}
