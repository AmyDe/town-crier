//go:build integration

package polling

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/AmyDe/town-crier/api-go/internal/applications"
	"github.com/AmyDe/town-crier/api-go/internal/planit"
	"github.com/AmyDe/town-crier/api-go/internal/platform/postgres/pgtest"
)

// TestNationalLane_CrossCycleCursorResume_RealPostgres proves ADR 0044's
// per-page checkpoint round-trips through the real poll_state table
// (cursor_different_start / cursor_next_index / high_water_mark — the
// EXISTING columns migrations 0003/0021 already added; no new migration)
// and that a genuinely SEPARATE handler + store instance (simulating a
// fresh poll cycle) resumes at the checkpointed index rather than
// re-treading the already-committed backlog.
func TestNationalLane_CrossCycleCursorResume_RealPostgres(t *testing.T) {
	ctx := context.Background()
	pool := pgtest.New(t)
	pgtest.Truncate(t, pool, "poll_state", "leases")
	state := NewPostgresPollStateStore(pool)

	watermark := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	cursor := &PollCursor{DifferentStart: watermark, NextIndex: 300}
	if err := state.Save(ctx, sentinelLaneA, watermark, watermark, cursor); err != nil {
		t.Fatalf("seed mid-drain state: %v", err)
	}

	ld := time.Date(2026, 7, 10, 0, 0, 0, 0, time.UTC)
	fetcher := newFakeNationalFetcher()
	fetcher.pages[200] = planit.FetchPageResult{
		From:         200,
		Applications: []applications.PlanningApplication{testApp("resumed", 300, ld)},
		HasMorePages: false,
	}
	apps := newFakeApps()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	clock := func() time.Time { return time.Date(2026, 7, 14, 12, 0, 0, 0, time.UTC) }

	// A fresh handler over a fresh Postgres store instance (same pool) — the
	// only thing carrying state across "cycles" is the database row.
	h := NewNationalLaneHandler(fetcher, NewPostgresPollStateStore(pool), apps, laneAOpts(), clock, logger)
	out := h.RunOnePage(ctx)
	if out.err != nil {
		t.Fatalf("RunOnePage: %v", out.err)
	}
	if len(fetcher.queries) != 1 || fetcher.queries[0].StartIndex != 200 {
		t.Fatalf("expected the resumed fetch at StartIndex 200 (300 - the 100-record resume overlap), got %+v", fetcher.queries)
	}
	if len(apps.upserts) != 1 || apps.upserts[0].Name != "resumed" {
		t.Fatalf("expected the resumed page's record ingested: got %+v", apps.upserts)
	}

	got, found, err := state.Get(ctx, sentinelLaneA)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if !found {
		t.Fatal("expected a persisted poll_state row")
	}
	if got.Cursor != nil {
		t.Errorf("cursor after completion: got %+v, want nil (the walk reached its boundary)", got.Cursor)
	}
	if !got.HighWaterMark.Equal(ld) {
		t.Errorf("HighWaterMark: got %v, want %v", got.HighWaterMark, ld)
	}
}

// TestInverseMaskLane_ScanCursorCrossCycleResume_RealPostgres is Lane C's
// analogue (§5, as amended by #1127): a mid-scan cursor (HighWaterMark =
// last_clean_scan_at, Cursor.DifferentStart = the scan's anchor date =
// today, Cursor.NextIndex = the within-scan offset — ADR 0044's reuse of the
// existing PollCursor shape, no migration) round-trips through real Postgres,
// and a fresh handler + store instance resumes the SAME scan at the
// checkpointed index.
func TestInverseMaskLane_ScanCursorCrossCycleResume_RealPostgres(t *testing.T) {
	ctx := context.Background()
	pool := pgtest.New(t)
	pgtest.Truncate(t, pool, "poll_state", "leases")
	state := NewPostgresPollStateStore(pool)

	now := time.Date(2026, 7, 14, 12, 0, 0, 0, time.UTC)
	today := time.Date(2026, 7, 14, 0, 0, 0, 0, time.UTC)
	lastCleanScanAt := time.Date(2026, 7, 12, 0, 0, 0, 0, time.UTC)
	cursor := &PollCursor{DifferentStart: today, NextIndex: 300}
	if err := state.Save(ctx, sentinelLaneC, lastCleanScanAt, lastCleanScanAt, cursor); err != nil {
		t.Fatalf("seed mid-scan state: %v", err)
	}

	newLD := now.Add(-2 * time.Hour)
	fetcher := newFakeInverseMaskFetcher()
	fetcher.pages[200] = planit.FetchPageResult{
		From:         200,
		Applications: []applications.PlanningApplication{lightApp("resumed/FUL", 99, "Permitted", newLD)},
		HasMorePages: false,
	}
	full := testApp("resumed", 99, newLD)
	full.UID = "resumed/FUL"
	fetcher.hydrated["resumed/FUL"] = full

	apps := newFakeApps()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	clock := func() time.Time { return now }

	h := NewInverseMaskLaneHandler(fetcher, NewPostgresPollStateStore(pool), apps, defaultInverseMaskOpts(), clock, logger)
	out := h.RunOnePage(ctx)
	if out.err != nil {
		t.Fatalf("RunOnePage: %v", out.err)
	}
	if len(fetcher.queries) != 1 || fetcher.queries[0].StartIndex != 200 {
		t.Fatalf("expected the resumed fetch at StartIndex 200 (checkpointed NextIndex 300 minus the 100-record resume overlap, GH#986), got %+v", fetcher.queries)
	}
	if fetcher.queries[0].WindowDays != 3 {
		t.Errorf("WindowDays: got %d, want 3 (last clean scan two days ago -> clamp(2+1, 2, 3))", fetcher.queries[0].WindowDays)
	}
	if len(apps.upserts) != 1 || apps.upserts[0].UID != "resumed/FUL" {
		t.Fatalf("expected the hydrated record ingested: got %+v", apps.upserts)
	}

	got, found, err := state.Get(ctx, sentinelLaneC)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if !found {
		t.Fatal("expected a persisted poll_state row")
	}
	if got.Cursor != nil {
		t.Errorf("cursor after the scan completes: got %+v, want nil", got.Cursor)
	}
	if !got.HighWaterMark.Equal(now) {
		t.Errorf("last_clean_scan_at: got %v, want now %v (clean scan stamps it)", got.HighWaterMark, now)
	}
}

// TestInverseMaskLane_CleanScanStampsLastCleanScanAt_RealPostgres proves the
// clean-scan completion path round-trips through real Postgres: a fresh scan
// (no active cursor) that reaches its last page with no 429 stamps
// last_clean_scan_at = now and leaves no cursor, on the EXISTING poll_state
// columns (no migration).
func TestInverseMaskLane_CleanScanStampsLastCleanScanAt_RealPostgres(t *testing.T) {
	ctx := context.Background()
	pool := pgtest.New(t)
	pgtest.Truncate(t, pool, "poll_state", "leases")
	state := NewPostgresPollStateStore(pool)

	priorCleanScan := time.Date(2026, 7, 8, 0, 0, 0, 0, time.UTC)
	if err := state.Save(ctx, sentinelLaneC, priorCleanScan, priorCleanScan, nil); err != nil {
		t.Fatalf("seed prior clean scan: %v", err)
	}

	fetcher := newFakeInverseMaskFetcher()
	fetcher.pages[0] = planit.FetchPageResult{From: 0, Applications: nil, HasMorePages: false}
	apps := newFakeApps()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	now := time.Date(2026, 7, 14, 12, 0, 0, 0, time.UTC)
	clock := func() time.Time { return now }

	h := NewInverseMaskLaneHandler(fetcher, NewPostgresPollStateStore(pool), apps, defaultInverseMaskOpts(), clock, logger)
	out := h.RunOnePage(ctx)
	if out.err != nil {
		t.Fatalf("RunOnePage: %v", out.err)
	}
	if len(fetcher.queries) != 1 || fetcher.queries[0].StartIndex != 0 || fetcher.queries[0].WindowDays != 3 {
		t.Fatalf("expected one fresh fetch at index 0 with WindowDays 3, got %+v", fetcher.queries)
	}

	got, found, err := state.Get(ctx, sentinelLaneC)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if !found {
		t.Fatal("expected a persisted poll_state row")
	}
	if got.Cursor != nil {
		t.Errorf("cursor: got %+v, want nil (clean scan)", got.Cursor)
	}
	if !got.HighWaterMark.Equal(now) {
		t.Errorf("last_clean_scan_at: got %v, want %v", got.HighWaterMark, now)
	}
}
