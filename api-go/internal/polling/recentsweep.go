// GH#1134 / ADR 0047: Lane E, the looping recent-window start_date sweep that
// backstops Lanes A/B. Where Lane A walks a churn-masked delta forward from a
// single watermark (and loses anything whose last_different slips below it),
// Lane E exhaustively re-reads the whole recent start_date band in bounded
// windows, newest first, comparing every PlanIt row against Postgres and
// hydrating only what diverges or is missing. A full lap takes about a week;
// then the cursor re-anchors to today and goes round again, forever. There is
// no Complete flag — the recent band never runs out.
//
// Modelled on BackfillHandler (backfill.go), NOT sharing code with it: ADR 0042
// makes "Lane D can never notify" a compile-time guarantee by hardwiring
// NewIngester(apps, nil, nil) and omitting WithFanOut entirely, and that
// guarantee is worth more than the duplication. Lane E DOES notify, behind an
// event-specific recency gate (recentsweep_gate.go) composed inside WithFanOut
// so the wiring site cannot hand it an ungated notifier.
package polling

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"github.com/AmyDe/town-crier/api-go/internal/applications"
	"github.com/AmyDe/town-crier/api-go/internal/planit"
)

// maxHydrationsPerSweepTurn bounds Lane E's FetchByUID fan-out within a single
// Run call, exactly as maxHydrationsPerPass bounds Lane C. A page of clustered
// missing/diverged rows could otherwise burst dozens of hydration requests and
// trip PlanIt's 429 threshold. Once the cap is reached the remaining rows are
// left alone, the page is checkpointed, and the turn returns cleanly (NOT an
// error) — a real backlog then drains across laps instead of bursting. A fixed
// safety rule, not an operator dial (mirrors planit.nationalPageSize).
const maxHydrationsPerSweepTurn = 10

// maxRecentSweepWindowWidthDays hard-caps the WindowWidthDays dial so nobody
// can set it to 90 and walk Lane E's first-page cost back toward PlanIt's ~45s
// cliff (ADR 0047's measured curve: 1.42s at 15 days, 17.17s at 90). A fixed
// safety rule, mirroring lanec.go's defaultMaxInverseMaskWindowDays.
const maxRecentSweepWindowWidthDays = 30

// RecentSweepState is Lane E's ENTIRE persisted state (a singleton row).
// LapAnchor is fixed for the lifetime of one lap — a floor recomputed from a
// moving "today" would recede as the lap runs, so the lap would chase it and
// never land (zero = never started). WindowEnd is the upper start_date bound of
// the window currently draining. CursorNextIndex is pagination progress within
// that window. LapsCompleted / LastLapCompletedAt track full verification laps.
// LastRunTime drives the planner's idle-interval pacing. There is no Complete
// field: Lane E never finishes.
type RecentSweepState struct {
	LapAnchor          time.Time
	WindowEnd          time.Time
	CursorNextIndex    int
	LapsCompleted      int
	LastLapCompletedAt time.Time
	LastRunTime        time.Time
}

// recentSweepFetcher is the consumer-side slice of the PlanIt client Lane E
// needs: one light recent-window page, and a full-record hydration fetch by
// uid. *planit.Client satisfies both.
type recentSweepFetcher interface {
	FetchRecentSweepPage(ctx context.Context, windowStart, windowEnd time.Time, startIndex int) (planit.FetchPageResult, error)
	FetchByUID(ctx context.Context, uid string) (planit.FetchPageResult, error)
}

// recentSweepStateAccess is the consumer-side slice of the recent-sweep-state
// store. *PostgresRecentSweepStateStore satisfies it.
type recentSweepStateAccess interface {
	Get(ctx context.Context) (RecentSweepState, error)
	Save(ctx context.Context, state RecentSweepState) error
}

// RecentSweepOptions tune Lane E's depth, window width, pace, and notification
// recency gate. DepthDays defaults to POLLING_LANE_A_MASK_DAYS so Lanes A, E
// and C partition the national start_date axis with no gap or overlap.
// WindowWidthDays is clamped to maxRecentSweepWindowWidthDays by the
// constructor. NotifyRecencyWindow feeds both fan-out decorators.
type RecentSweepOptions struct {
	DepthDays           int
	WindowWidthDays     int
	MaxPagesPerCycle    int
	NotifyRecencyWindow time.Duration
}

// RecentSweepHandler runs ADR 0047's Lane E: a national, date-windowed
// recent-band sweep that loops backward through bounded start_date windows,
// verifies each PlanIt row against Postgres, hydrates only what diverges or is
// missing, and fans out — behind the recency gate — for anything it surfaces.
type RecentSweepHandler struct {
	fetcher  recentSweepFetcher
	state    recentSweepStateAccess
	apps     applicationStore
	ingester *Ingester
	opts     RecentSweepOptions
	now      func() time.Time
	logger   *slog.Logger

	metrics metricsRecorder
}

// NewRecentSweepHandler wires Lane E. now is injected so tests pin the clock.
// WindowWidthDays is clamped into [1, maxRecentSweepWindowWidthDays] here — a
// misconfigured value can never widen the query toward PlanIt's cliff. The
// ingester starts with nil fan-out collaborators (ingestion-only); WithFanOut
// wires the gated ones.
func NewRecentSweepHandler(
	fetcher recentSweepFetcher,
	state recentSweepStateAccess,
	apps applicationStore,
	opts RecentSweepOptions,
	now func() time.Time,
	logger *slog.Logger,
) *RecentSweepHandler {
	if opts.WindowWidthDays < 1 {
		opts.WindowWidthDays = 1
	}
	if opts.WindowWidthDays > maxRecentSweepWindowWidthDays {
		opts.WindowWidthDays = maxRecentSweepWindowWidthDays
	}
	return &RecentSweepHandler{
		fetcher:  fetcher,
		state:    state,
		apps:     apps,
		ingester: NewIngester(apps, nil, nil),
		opts:     opts,
		now:      now,
		logger:   logger,
	}
}

// WithFanOut wraps the fan-out collaborators in Lane E's event-specific
// recency decorators ITSELF, then rebuilds its Ingester over them. The wiring
// site cannot hand Lane E an ungated notifier, because the gate is applied
// inside this setter rather than at the call site — the same structural
// instinct ADR 0042 used when it omitted WithFanOut from Lane D, applied to a
// lane that does need to notify. Returns the handler for chaining.
func (h *RecentSweepHandler) WithFanOut(decision DecisionDispatcher, enqueuer NotificationEnqueuer) *RecentSweepHandler {
	if h.ingester == nil {
		h.ingester = &Ingester{}
	}
	h.ingester.decision = recencyGatedDispatcher{inner: decision, window: h.opts.NotifyRecencyWindow, now: h.now}
	h.ingester.enqueuer = recencyGatedEnqueuer{inner: enqueuer, window: h.opts.NotifyRecencyWindow, now: h.now}
	return h
}

// WithMetrics wires the metrics recorder Lane E records its per-run
// applications-ingested count on, mirroring the other lanes' WithMetrics.
// Returns the handler for chaining.
func (h *RecentSweepHandler) WithMetrics(rec metricsRecorder) *RecentSweepHandler {
	h.metrics = rec
	return h
}

// recorder returns a non-nil recorder so call sites can record
// unconditionally, mirroring the other lanes' recorder.
func (h *RecentSweepHandler) recorder() metricsRecorder {
	if h.metrics == nil {
		return noopMetrics{}
	}
	return h.metrics
}

// recentSweepOutcome is one Run call's result, carried onto the telemetry span
// and mapped into commonLaneOutcome by NationalPollHandler.execOnePage.
type recentSweepOutcome struct {
	pages           int
	recordsSeen     int
	recordsIngested int
	hydrations      int
	hydrationCapHit bool
	rateLimited     bool
	retryAfter      *time.Duration
	err             error
	// planitOrigin mirrors laneOutcome.planitOrigin: true only when err came
	// from a PlanIt fetch call (page fetch or hydration), false for a
	// state-read, ingest, or state-save error — all genuine state-store
	// problems unrelated to PlanIt.
	planitOrigin  bool
	planitTotal   *int
	windowEnd     time.Time
	lapAnchor     time.Time
	lapsCompleted int
}

// Run does one Lane E turn: read state (start a fresh lap if never run), then
// fetch up to MaxPagesPerCycle light pages of the current window, verifying
// each row against Postgres and hydrating only what diverges or is missing.
// State is persisted after every successful page. A window that fully drains
// slides back by WindowWidthDays; when the cursor crosses LapAnchor-DepthDays
// the lap ends and a fresh one is cut at today.
//
// A fetch error, hydration error or ingest error stops the turn WITHOUT
// persisting the failed page (prior pages in the same turn stay persisted). A
// 429 (page fetch or hydration) records rateLimited/retryAfter and stops the
// turn without an error. Reaching maxHydrationsPerSweepTurn checkpoints the
// page, sets the cap-hit flag, and returns cleanly.
func (h *RecentSweepHandler) Run(ctx context.Context) recentSweepOutcome {
	ctx, span := otel.Tracer(tracerName).Start(ctx, "PlanIt Lane E recent-window sweep")
	defer span.End()

	now := h.now().UTC()
	var out recentSweepOutcome

	state, err := h.state.Get(ctx)
	if err != nil {
		out.err = fmt.Errorf("lane E: read recent sweep state: %w", err)
		span.SetAttributes(attribute.String("poll.lane", string(LaneE)))
		return out
	}

	if state.LapAnchor.IsZero() {
		state.LapAnchor = truncateToDate(now)
		state.WindowEnd = state.LapAnchor
		state.CursorNextIndex = 0
	}

	floor := state.LapAnchor.AddDate(0, 0, -h.opts.DepthDays)

pageLoop:
	for out.pages < h.opts.MaxPagesPerCycle {
		if ctx.Err() != nil {
			break
		}

		windowStart := state.WindowEnd.AddDate(0, 0, -h.opts.WindowWidthDays)

		res, ferr := h.fetcher.FetchRecentSweepPage(ctx, windowStart, state.WindowEnd, state.CursorNextIndex)
		if ferr != nil {
			var rl *planit.RateLimitError
			if errors.As(ferr, &rl) {
				out.rateLimited = true
				out.retryAfter = rl.RetryAfter
			} else {
				out.err = ferr
				out.planitOrigin = true
			}
			break pageLoop
		}

		out.pages++
		if out.planitTotal == nil {
			out.planitTotal = res.Total
		}
		out.recordsSeen += len(res.Applications)

		capHit := false
		for _, light := range res.Applications {
			if perr := h.processRow(ctx, light, &capHit, &out); perr != nil {
				out.err = perr
				break pageLoop
			}
			if out.rateLimited {
				break pageLoop
			}
			if capHit {
				break
			}
		}

		// Checkpoint this page. A drained window slides back; crossing the
		// floor ends the lap and cuts a fresh anchor at today.
		state.CursorNextIndex += len(res.Applications)
		state.LastRunTime = now

		if !res.HasMorePages {
			state.WindowEnd = windowStart
			state.CursorNextIndex = 0
			if !state.WindowEnd.After(floor) {
				state.LapsCompleted++
				state.LastLapCompletedAt = now
				state.LapAnchor = truncateToDate(now)
				state.WindowEnd = state.LapAnchor
				state.CursorNextIndex = 0
				floor = state.LapAnchor.AddDate(0, 0, -h.opts.DepthDays)
			}
		}

		if serr := h.state.Save(ctx, state); serr != nil {
			out.err = serr
			break pageLoop
		}

		if capHit {
			out.hydrationCapHit = true
			break pageLoop
		}
	}

	out.windowEnd = state.WindowEnd
	out.lapAnchor = state.LapAnchor
	out.lapsCompleted = state.LapsCompleted

	h.recordRunMetrics(ctx, out)
	h.setSpanAttributes(span, out)
	return out
}

// processRow verifies one light row against Postgres (scoped by area_id — a
// national query cannot diff by a bare uid, which is only unique within one
// authority) and hydrates it when it is missing or when inverseMaskDiffers
// reports a change. Once maxHydrationsPerSweepTurn is reached, a row that would
// otherwise hydrate is left alone and *capHit is set so the caller stops the
// turn after checkpointing. A local read failure or an ingest failure is a
// hard stop (returned as an error).
func (h *RecentSweepHandler) processRow(ctx context.Context, light applications.PlanningApplication, capHit *bool, out *recentSweepOutcome) error {
	authorityCode := strconv.Itoa(light.AreaID)
	existing, found, gerr := h.apps.GetByUID(ctx, light.UID, authorityCode)
	if gerr != nil {
		return fmt.Errorf("lane E: read existing application %q (authority %s): %w", light.UID, authorityCode, gerr)
	}
	if found && !inverseMaskDiffers(existing, light) {
		return nil
	}
	if out.hydrations >= maxHydrationsPerSweepTurn {
		*capHit = true
		return nil
	}
	out.hydrations++
	return h.hydrate(ctx, light.UID, light.AreaID, out)
}

// hydrate fetches one row's full record by uid and feeds it through the
// gated Ingester. wantAreaID guards against PlanIt's id_match lookup crossing
// authorities: only a hydrated record whose AreaID matches the light row that
// flagged it is ingested; any other match is logged and skipped, exactly as
// Lane C does. A 429 is recorded on out (the caller stops the turn); any other
// fetch error is a PlanIt-origin hard stop; an Ingest failure is a
// state-store hard stop.
func (h *RecentSweepHandler) hydrate(ctx context.Context, uid string, wantAreaID int, out *recentSweepOutcome) error {
	full, err := h.fetcher.FetchByUID(ctx, uid)
	if err != nil {
		var rl *planit.RateLimitError
		if errors.As(err, &rl) {
			out.rateLimited = true
			out.retryAfter = rl.RetryAfter
			return nil
		}
		out.planitOrigin = true
		return fmt.Errorf("lane E: hydration fetch %q: %w", uid, err)
	}
	for _, app := range full.Applications {
		if app.UID != uid || app.AreaID != wantAreaID {
			continue
		}
		if ierr := h.ingester.Ingest(ctx, app); ierr != nil {
			return fmt.Errorf("lane E: hydrated ingest %q: %w", uid, ierr)
		}
		out.recordsIngested++
		return nil
	}
	h.logger.WarnContext(ctx, "lane E: hydration fetch returned no matching record", "uid", uid, "areaId", wantAreaID)
	return nil
}

// recordRunMetrics records Lane E's per-run applications-ingested count
// (tagged "E") and, on a 429, the rate-limit counter and Retry-After value.
// Lane E has no forward watermark, so it does not record
// OldestHighWaterMarkAge — same omission and rationale as Lane D.
func (h *RecentSweepHandler) recordRunMetrics(ctx context.Context, out recentSweepOutcome) {
	rec := h.recorder()
	rec.ApplicationsIngested(ctx, out.recordsIngested, string(LaneE))
	if out.rateLimited {
		rec.RateLimited(ctx, string(LaneE))
		if out.retryAfter == nil {
			rec.RetryAfterSeconds(ctx, 0, string(LaneE), 0, false)
		} else {
			rec.RetryAfterSeconds(ctx, out.retryAfter.Seconds(), string(LaneE), 0, true)
		}
	}
}

// setSpanAttributes stamps the "PlanIt Lane E recent-window sweep" span with
// Lane E's telemetry set (ADR 0047 §8), mirroring the other lanes'
// setSpanAttributes. poll.lane is required because planitLaneSlowQuery groups
// by Name and lane.
func (h *RecentSweepHandler) setSpanAttributes(span trace.Span, out recentSweepOutcome) {
	attrs := []attribute.KeyValue{
		attribute.String("poll.lane", string(LaneE)),
		attribute.Int("poll.records_ingested", out.recordsIngested),
		attribute.Int("lane_e.pages", out.pages),
		attribute.Int("lane_e.records_seen", out.recordsSeen),
		attribute.Int("lane_e.hydrations", out.hydrations),
		attribute.Bool("lane_e.hydration_cap_hit", out.hydrationCapHit),
		attribute.String("lane_e.window_end", formatSweepDate(out.windowEnd)),
		attribute.String("lane_e.lap_anchor", formatSweepDate(out.lapAnchor)),
		attribute.Int("lane_e.laps_completed", out.lapsCompleted),
		attribute.Bool("lane_e.rate_limited", out.rateLimited),
	}
	if out.planitTotal != nil {
		attrs = append(attrs, attribute.Int("planit.total", *out.planitTotal))
	}
	if out.err != nil {
		attrs = append(attrs, attribute.String("lane_e.error", out.err.Error()))
	}
	span.SetAttributes(attrs...)
}

// formatSweepDate renders a Lane E window/anchor date as an RFC3339 full-date
// (YYYY-MM-DD), or "" for the zero time.
func formatSweepDate(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format("2006-01-02")
}
