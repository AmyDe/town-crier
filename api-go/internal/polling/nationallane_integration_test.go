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

// TestInverseMaskLane_EpochCursorCrossCycleResume_RealPostgres is Lane C's
// analogue: a mid-epoch cursor (HighWaterMark = pinned epoch_upper,
// Cursor.DifferentStart = epoch_lower, Cursor.NextIndex = the ascending
// offset — ADR 0044's reuse of the existing PollCursor shape, no migration)
// round-trips through real Postgres, and a fresh handler + store instance
// resumes the SAME epoch at the checkpointed index.
func TestInverseMaskLane_EpochCursorCrossCycleResume_RealPostgres(t *testing.T) {
	ctx := context.Background()
	pool := pgtest.New(t)
	pgtest.Truncate(t, pool, "poll_state", "leases")
	state := NewPostgresPollStateStore(pool)

	epochLower := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	epochUpper := time.Date(2026, 7, 10, 0, 0, 0, 0, time.UTC)
	cursor := &PollCursor{DifferentStart: epochLower, NextIndex: 300}
	if err := state.Save(ctx, sentinelLaneC, epochUpper, epochUpper, cursor); err != nil {
		t.Fatalf("seed mid-epoch state: %v", err)
	}

	newLD := epochLower.Add(2 * time.Hour)
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
	clock := func() time.Time { return time.Date(2026, 7, 14, 12, 0, 0, 0, time.UTC) }

	h := NewInverseMaskLaneHandler(fetcher, NewPostgresPollStateStore(pool), apps, defaultInverseMaskOpts(), clock, logger)
	out := h.RunOnePage(ctx)
	if out.err != nil {
		t.Fatalf("RunOnePage: %v", out.err)
	}
	if len(fetcher.queries) != 1 || fetcher.queries[0].StartIndex != 200 {
		t.Fatalf("expected the resumed fetch at StartIndex 200 (checkpointed NextIndex 300 minus the 100-record resume overlap, GH#986), got %+v", fetcher.queries)
	}
	if !fetcher.queries[0].EpochLower.Equal(epochLower) {
		t.Errorf("EpochLower: got %v, want the active epoch's floor %v", fetcher.queries[0].EpochLower, epochLower)
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
		t.Errorf("cursor after the epoch drains: got %+v, want nil", got.Cursor)
	}
	if !got.HighWaterMark.Equal(epochUpper) {
		t.Errorf("HighWaterMark (pinned epoch_upper): got %v, want unchanged %v", got.HighWaterMark, epochUpper)
	}
}

// TestInverseMaskLane_ContiguousEpochTiling_RealPostgres proves the "no gap"
// tiling guarantee survives a real Postgres round trip: once a prior epoch's
// drained ceiling is persisted (no active cursor), a fresh handler + store
// instance anchors the next epoch with EXACTLY that ceiling as its floor.
func TestInverseMaskLane_ContiguousEpochTiling_RealPostgres(t *testing.T) {
	ctx := context.Background()
	pool := pgtest.New(t)
	pgtest.Truncate(t, pool, "poll_state", "leases")
	state := NewPostgresPollStateStore(pool)

	priorCeiling := time.Date(2026, 7, 10, 0, 0, 0, 0, time.UTC)
	if err := state.Save(ctx, sentinelLaneC, priorCeiling, priorCeiling, nil); err != nil {
		t.Fatalf("seed drained epoch: %v", err)
	}

	fetcher := newFakeInverseMaskFetcher()
	fetcher.pages[0] = planit.FetchPageResult{From: 0, Applications: nil, HasMorePages: false}
	apps := newFakeApps()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	wantNewCeiling := time.Date(2026, 7, 14, 12, 0, 0, 0, time.UTC)
	clock := func() time.Time { return wantNewCeiling }

	h := NewInverseMaskLaneHandler(fetcher, NewPostgresPollStateStore(pool), apps, defaultInverseMaskOpts(), clock, logger)
	out := h.RunOnePage(ctx)
	if out.err != nil {
		t.Fatalf("RunOnePage: %v", out.err)
	}
	if len(fetcher.queries) != 1 || !fetcher.queries[0].EpochLower.Equal(priorCeiling) {
		t.Fatalf("expected the new epoch's floor to equal the prior epoch's ceiling %v, got %+v", priorCeiling, fetcher.queries)
	}

	got, found, err := state.Get(ctx, sentinelLaneC)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if !found {
		t.Fatal("expected a persisted poll_state row")
	}
	if !got.HighWaterMark.Equal(wantNewCeiling) {
		t.Errorf("new epoch_upper: got %v, want %v", got.HighWaterMark, wantNewCeiling)
	}
}
