// ADR 0041 / GH#962 (churn-masked axis) + ADR 0044 (resumable, checkpointed
// execution model). This file replaces the per-authority drain (handler.go,
// left compiling but unwired — see cmd/worker/main.go) with a single
// national query per lane. ADR 0041 gave each lane ONE global watermark; ADR
// 0044 adds per-page checkpointing on top (the existing PollCursor,
// reused — no schema migration) and replaces the old "each lane fully drains
// itself per cycle" model with a one-page-at-a-time executor driven by
// NationalPollHandler.Handle's planner loop (planner.go). State is persisted
// in the EXISTING poll_state table via a reserved sentinel authority_id.
package polling

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"github.com/AmyDe/town-crier/api-go/internal/planit"
)

// Sentinel poll_state.authority_id values reserved for the ADR 0041/0044
// lanes' global watermark bookkeeping. Real PlanIt authority ids are always
// positive (authorities.Lookup never emits <= 0), so a sentinel row can never
// collide with a real authority's poll state.
const (
	sentinelLaneA = -1
	sentinelLaneB = -2
	sentinelLaneC = -3
)

// LaneName tags which lane produced a span/result: "A" (new applications),
// "B" (decisions), "C" (inverse-mask reconciliation), or "D" (historical
// backfill — see LaneD in planner.go).
type LaneName string

// LaneA, LaneB, and LaneC are three of the four ADR 0044 lanes (LaneD lives
// in planner.go, tagged into this same vocabulary).
const (
	LaneA LaneName = "A"
	LaneB LaneName = "B"
	LaneC LaneName = "C"
)

// sentinelIDForLane maps a lane to its reserved poll_state.authority_id, so a
// lane handler's caller never needs to know or pass the sentinel directly —
// removing a footgun where a lane and its watermark row could be mismatched.
func sentinelIDForLane(lane LaneName) int {
	switch lane {
	case LaneB:
		return sentinelLaneB
	case LaneC:
		return sentinelLaneC
	case LaneA:
		return sentinelLaneA
	default:
		return sentinelLaneA
	}
}

// nationalDeltaFetcher is the consumer-side slice of the PlanIt client a
// national lane needs: one descending page of the churn-masked delta query.
// *planit.Client satisfies it.
type nationalDeltaFetcher interface {
	FetchNationalDeltaPage(ctx context.Context, q planit.NationalDeltaQuery) (planit.FetchPageResult, error)
}

// laneWatermarkStore persists ONE lane's global delta watermark plus its
// resumable per-page cursor in the existing poll_state table via a reserved
// sentinel authority_id (see the package doc above): HighWaterMark holds the
// watermark (Lane A/B: the descending delta watermark; Lane C: the pinned
// epoch_upper — see lanec.go), LastPollTime the lane's last-run time, Cursor
// the active resume position (Lane A/B: an insurance checkpoint for a
// multi-page walk; Lane C: DifferentStart doubles as epoch_lower and
// NextIndex as the ascending record offset — ADR 0044's reuse of the
// existing PollCursor shape, no migration). It is a thin wrapper over the
// existing pollStateAccess Get/Save — no new store, no new columns.
type laneWatermarkStore struct {
	state      pollStateAccess
	sentinelID int
}

func newLaneWatermarkStore(state pollStateAccess, sentinelID int) *laneWatermarkStore {
	return &laneWatermarkStore{state: state, sentinelID: sentinelID}
}

// get returns the lane's persisted watermark, last-run time, and cursor. All
// three are zero/nil when the lane has never run (no sentinel row yet).
func (s *laneWatermarkStore) get(ctx context.Context) (watermark, lastRun time.Time, cursor *PollCursor, err error) {
	st, found, err := s.state.Get(ctx, s.sentinelID)
	if err != nil {
		return time.Time{}, time.Time{}, nil, err
	}
	if !found {
		return time.Time{}, time.Time{}, nil, nil
	}
	return st.HighWaterMark, st.LastPollTime, st.Cursor, nil
}

// save persists the lane's watermark, this run's timestamp, and its cursor.
// A nil cursor clears any previously active cursor.
func (s *laneWatermarkStore) save(ctx context.Context, lastRun, watermark time.Time, cursor *PollCursor) error {
	return s.state.Save(ctx, s.sentinelID, lastRun, watermark, cursor)
}

// NationalLaneOptions configure one of the national delta lanes (A or B —
// Lane C has its own InverseMaskOptions in lanec.go).
type NationalLaneOptions struct {
	// Lane tags telemetry and selects the sentinel watermark row ("A" or "B").
	Lane LaneName
	// Mask selects the churn-mask query parameter: planit.MaskStartDate (Lane
	// A) or planit.MaskDecidedStart (Lane B).
	Mask planit.MaskParam
	// MaskWindow is how far back the churn mask reaches (today - MaskWindow).
	// A config dial, not a correctness boundary (ADR 0041): Lane C is the
	// backstop for anything a mask misses.
	MaskWindow time.Duration
	// MaxPages bounds how many pages of this lane NationalPollHandler.Handle
	// will run within a single Handle call (nil = unbounded — Lane A: the
	// national delta is measured at ~6 pages/day). Non-nil hard-caps the
	// PER-CYCLE page count (Lane B: 20 pages/cycle — decision volume is
	// unmeasured pre-cutover, and this cap must not be removed); it is NOT
	// read by RunOnePage itself — ADR 0044's checkpointed model means a
	// lane's walk resumes across cycles with no lost progress, so the old
	// "per-Run() cap, safe because the next cycle just re-walks from
	// scratch" reasoning no longer applies. Handle enforces this by
	// excluding the lane from planner candidacy for the REST of the current
	// cycle once its own per-cycle page count is reached (see
	// NationalPollHandler.loadPlannerState) — the walk resumes exactly where
	// it left off on the next cycle via the persisted cursor, so the cap
	// still bounds decision volume per cycle without ever losing progress or
	// busy-looping.
	MaxPages *int
}

// NationalLaneHandler runs one national delta lane (A or B) as a one-page
// executor (ADR 0044): RunOnePage builds the query from the lane's
// persisted watermark and resume cursor, fetches exactly ONE page, ingests
// it, and persists the advanced checkpoint. The page LOOP that used to walk
// a lane to its boundary in a single call (ADR 0041) now lives up in
// NationalPollHandler.Handle, which calls RunOnePage repeatedly (via the
// planner) until the walk completes, is rate-limited, or errors — so a 429
// costs at most the one in-flight page, and the next cycle resumes exactly
// where this one stopped.
type NationalLaneHandler struct {
	fetcher   nationalDeltaFetcher
	watermark *laneWatermarkStore
	ingester  *Ingester
	opts      NationalLaneOptions
	now       func() time.Time
	logger    *slog.Logger

	// metrics records the towncrier.polling.* business KPIs for this lane's
	// runs, wired via WithMetrics, mirroring PollPlanItHandler.metrics. nil
	// until wired (the no-metrics default), so the many ingestion-only tests
	// and call sites that don't supply one record nothing.
	metrics metricsRecorder
}

// NewNationalLaneHandler wires a national lane. now is injected so tests pin
// the clock.
func NewNationalLaneHandler(
	fetcher nationalDeltaFetcher,
	state pollStateAccess,
	apps applicationStore,
	opts NationalLaneOptions,
	now func() time.Time,
	logger *slog.Logger,
) *NationalLaneHandler {
	return &NationalLaneHandler{
		fetcher:   fetcher,
		watermark: newLaneWatermarkStore(state, sentinelIDForLane(opts.Lane)),
		ingester:  NewIngester(apps, nil, nil),
		opts:      opts,
		now:       now,
		logger:    logger,
	}
}

// WithFanOut wires the notification fan-out collaborators onto this lane's
// ingests, mirroring PollPlanItHandler.WithFanOut (including its nil-ingester
// guard, so calling this on a zero-value NationalLaneHandler — as a wiring
// test does — never panics). Returns the handler for chaining.
func (h *NationalLaneHandler) WithFanOut(decision DecisionDispatcher, enqueuer NotificationEnqueuer) *NationalLaneHandler {
	if h.ingester == nil {
		h.ingester = &Ingester{}
	}
	h.ingester.decision = decision
	h.ingester.enqueuer = enqueuer
	return h
}

// WithMetrics wires the metrics recorder this lane records its per-run
// business KPIs on, mirroring PollPlanItHandler.WithMetrics. A
// post-construction setter, so the many ingestion-only call sites and tests
// are unaffected; cmd/worker calls it once per lane after construction.
// Returns the handler for chaining.
func (h *NationalLaneHandler) WithMetrics(rec metricsRecorder) *NationalLaneHandler {
	h.metrics = rec
	return h
}

// recorder returns a non-nil recorder so call sites can record
// unconditionally, mirroring PollPlanItHandler.recorder.
func (h *NationalLaneHandler) recorder() metricsRecorder {
	if h.metrics == nil {
		return noopMetrics{}
	}
	return h.metrics
}

// laneOutcome is one lane page's result, carried both into PollPlanItResult
// (by the caller) and onto the lane's telemetry span. Lane C (lanec.go)
// reuses this same shape: watermarkBefore/After there hold epoch_lower and
// the epoch_upper in effect after the call, rather than a literal delta
// watermark.
type laneOutcome struct {
	recordsSeen     int
	recordsIngested int
	pages           int
	rateLimited     bool
	retryAfter      *time.Duration
	err             error
	// timedOut is true when err came from a PlanIt fetch call (page fetch or
	// hydration) whose client-side timeout expired — never from a
	// watermark/cursor persistence error, which is unrelated to PlanIt. It
	// lets Handle's loop distinguish "PlanIt needs space" (TerminationTimeout)
	// from a genuine natural completion (tc-pmh5y).
	timedOut        bool
	planitTotal     *int
	watermarkBefore time.Time
	watermarkAfter  time.Time
	// capHit is retained for telemetry continuity (poll.cap_hit) but is
	// always false from RunOnePage under the ADR 0044 model — MaxPages is
	// now enforced a level up, by NationalPollHandler excluding a capped
	// lane from planner candidacy, not by the one-page executor itself (see
	// NationalLaneOptions.MaxPages).
	capHit bool
}

// isTimeoutError reports whether err is (or wraps, via %w) a net.Error whose
// Timeout() reports true — the client-side-timeout shape a PlanIt fetch
// produces once its retries are exhausted (internal/planit/client.go's
// sendWithThrottle wraps the underlying *url.Error as
// "planit request failed: %w", which errors.As unwraps through).
// context.DeadlineExceeded alone is not a net.Error; the *url.Error (or
// similar) that wraps it is what implements Timeout().
func isTimeoutError(err error) bool {
	var netErr net.Error
	if errors.As(err, &netErr) {
		return netErr.Timeout()
	}
	return false
}

// RunOnePage executes exactly ONE page of this lane's descending delta walk
// (ADR 0044 §1/§2): read the persisted watermark and resume cursor, build
// the query, fetch one page, ingest it, and persist the advanced checkpoint.
//
// A record whose LastDifferent is at or before the watermark was already
// ingested by a prior walk (the watermark is set to that walk's max ingested
// value) — paging stops there without re-ingesting it. Every record strictly
// newer that shares its page with the boundary record has already been
// ingested earlier in this same page's iteration (descending order means it
// comes first), so nothing between the old watermark and the new one is
// dropped.
//
// "That walk's max ingested value" is the WHOLE walk's maximum, which for a
// descending walk is always the very first record of the walk's very first
// page — never a later page's own max (GH#983: a multi-page walk used to set
// the watermark to the BOUNDARY page's max instead, since maxIngested was
// scoped to one page and went out of scope across the ADR 0044 checkpoint).
// walkHead captures that first-page value once, on the fresh start
// (startIndex == 0), and carries it through every resume via
// PollCursor.WalkHead so a later page's completion still uses it.
//
// The watermark advances ONLY when THIS page reaches the boundary or the
// last page (a clean completion of the whole walk): only then is every
// record between the old watermark and the new one accounted for. Any
// earlier stop (fetch error, 429, or simply "more pages remain") freezes the
// watermark and persists a resume cursor instead — the next call to
// RunOnePage (this cycle or a later one, via NationalPollHandler.Handle's
// planner loop) resumes at the checkpointed index rather than re-walking
// from the start. A record actually received always advances the cursor
// (truncation-immune, GH#955/tc-nlvpz); nothing beyond the last SUCCESSFUL
// page is ever persisted, so a page that errors mid-ingest leaves the
// PREVIOUS page's checkpoint standing (a retry simply re-fetches this page
// from its start — free, via Ingester's dedup gate).
//
// A lane with no prior watermark (never run) does NOT walk this way at all —
// see seed.
func (h *NationalLaneHandler) RunOnePage(ctx context.Context) laneOutcome {
	ctx, span := otel.Tracer(tracerName).Start(ctx, "PlanIt national lane poll")
	defer span.End()

	now := h.now().UTC()
	var out laneOutcome

	watermarkBefore, _, cursor, err := h.watermark.get(ctx)
	if err != nil {
		out.err = fmt.Errorf("lane %s: read watermark: %w", h.opts.Lane, err)
		span.SetAttributes(attribute.String("poll.lane", string(h.opts.Lane)))
		return out
	}
	out.watermarkBefore = watermarkBefore

	maskCutoff := truncateToDate(now.Add(-h.opts.MaskWindow))

	// A lane with no prior watermark has never run. Walking the mask window
	// here — as a normal delta walk would, since "> zero time" is true for
	// every real record — would be a historical backfill (forbidden: ADR 0041
	// rule 2). Seed instead: read PlanIt's current head from a single
	// page-0 fetch and persist it, ingesting nothing.
	if watermarkBefore.IsZero() {
		return h.seed(ctx, span, now, maskCutoff)
	}

	prefilterDate := truncateToDate(watermarkBefore)

	// A cursor is active only while it still anchors the current watermark's
	// calendar date; once the watermark advances, a stale cursor from a
	// prior walk is ignored (fresh start at index 0) — the same
	// activeCursor/sameDate staleness guard handler.go's per-authority drain
	// uses, reused here at national-lane scope (ADR 0044).
	startIndex := 0
	if cursor != nil && sameDate(cursor.DifferentStart, watermarkBefore) {
		startIndex = max(0, cursor.NextIndex-resumeOverlapRecords)
	}

	res, ferr := h.fetcher.FetchNationalDeltaPage(ctx, planit.NationalDeltaQuery{
		DifferentStart: prefilterDate,
		Mask:           h.opts.Mask,
		MaskCutoff:     maskCutoff,
		StartIndex:     startIndex,
	})
	if ferr != nil {
		var rl *planit.RateLimitError
		if errors.As(ferr, &rl) {
			out.rateLimited = true
			out.retryAfter = rl.RetryAfter
		} else {
			out.err = ferr
			out.timedOut = isTimeoutError(ferr)
		}
		out.watermarkAfter = watermarkBefore
		h.recordRunMetrics(ctx, out, now)
		h.setSpanAttributes(span, out, watermarkBefore, prefilterDate, false)
		return out
	}

	out.pages = 1
	out.planitTotal = res.Total
	out.recordsSeen = len(res.Applications)

	// Diagnostic logging for the frozen-watermark investigation (tc-h2tcx):
	// no control-flow or behavioral change, purely additive. planitTotal is
	// dereferenced behind a nil check (res.Total is a nilable *int, same as
	// the setSpanAttributes convention above) rather than logged as a raw
	// pointer.
	planitTotal := 0
	if res.Total != nil {
		planitTotal = *res.Total
	}
	logArgs := []any{
		"lane", string(h.opts.Lane),
		"watermarkBefore", watermarkBefore,
		"differentStart", prefilterDate,
		"maskCutoff", maskCutoff,
		"startIndex", startIndex,
		"planitTotal", planitTotal,
		"recordsSeen", len(res.Applications),
	}
	if len(res.Applications) > 0 {
		first := res.Applications[0]
		last := res.Applications[len(res.Applications)-1]
		logArgs = append(logArgs,
			"firstUID", first.UID,
			"firstLastDifferent", first.LastDifferent,
			"lastUID", last.UID,
			"lastLastDifferent", last.LastDifferent,
		)
	}
	h.logger.InfoContext(ctx, "lane delta page fetched", logArgs...)

	// walkHead is this DESCENDING walk's true maximum LastDifferent (GH#983):
	// always the first record of the walk's first page (index 0), never a
	// later page's own max. A fresh walk (startIndex == 0) captures it
	// directly off this page; a resumed walk (startIndex != 0) carries it
	// forward from the persisted cursor, since the page that captured it is
	// long gone by the time a later page completes the walk. It stays the
	// zero value (unset) on a fresh walk's empty page 0, or when resuming a
	// pre-migration cursor that never recorded one -- see the completion
	// switch below, which degrades to maxIngested in that case.
	var walkHead time.Time
	if startIndex == 0 {
		if len(res.Applications) > 0 {
			walkHead = res.Applications[0].LastDifferent
		}
	} else if cursor != nil {
		walkHead = cursor.WalkHead
	}

	reachedBoundary := false
	var maxIngested time.Time
	for _, app := range res.Applications {
		if !app.LastDifferent.After(watermarkBefore) {
			reachedBoundary = true
			break
		}
		if ierr := h.ingester.Ingest(ctx, app); ierr != nil {
			out.err = ierr
			out.watermarkAfter = watermarkBefore
			h.recordRunMetrics(ctx, out, now)
			h.setSpanAttributes(span, out, watermarkBefore, prefilterDate, false)
			return out
		}
		out.recordsIngested++
		if app.LastDifferent.After(maxIngested) {
			maxIngested = app.LastDifferent
		}
	}

	nextIndex := startIndex + len(res.Applications)
	complete := reachedBoundary || !res.HasMorePages

	if complete {
		// walkHead wins whenever it is known: it is the WHOLE walk's true
		// maximum (GH#983), never just this page's. maxIngested is the
		// legacy-degrade fallback (acceptance criterion 4) for a pre-migration
		// cursor that never captured a walk head -- today's exact behaviour,
		// self-healing the moment the next fresh walk (startIndex == 0)
		// captures a real one.
		newWatermark := watermarkBefore
		switch {
		case !walkHead.IsZero():
			newWatermark = walkHead
		case !maxIngested.IsZero():
			newWatermark = maxIngested
		}
		out.watermarkAfter = newWatermark
		if serr := h.watermark.save(ctx, now, newWatermark, nil); serr != nil && out.err == nil {
			out.err = serr
		}
	} else {
		out.watermarkAfter = watermarkBefore
		newCursor := &PollCursor{DifferentStart: prefilterDate, NextIndex: nextIndex, KnownTotal: res.Total, WalkHead: walkHead}
		if serr := h.watermark.save(ctx, now, watermarkBefore, newCursor); serr != nil && out.err == nil {
			out.err = serr
		}
	}

	h.recordRunMetrics(ctx, out, now)
	h.setSpanAttributes(span, out, watermarkBefore, prefilterDate, false)
	return out
}

// seed handles a lane's first-ever run (no prior watermark): forward-flow
// only, never a historical sweep (ADR 0041 rule 2). It fetches PlanIt's
// current head from a SINGLE page-0 fetch — never paging further — and
// persists that as the watermark without ingesting anything, so the cutover
// starts from "now" rather than replaying the whole masked window. An empty
// page 0 (nothing currently matches the masked window) seeds the watermark to
// now() instead, so a quiet mask window still leaves the lane seeded rather
// than permanently stuck re-attempting the seed. A page-0 fetch error or 429
// seeds NOTHING — the lane stays unseeded and the next cycle retries the seed
// (bounded to one extra request/cycle, harmless).
func (h *NationalLaneHandler) seed(ctx context.Context, span trace.Span, now, maskCutoff time.Time) laneOutcome {
	var out laneOutcome

	res, ferr := h.fetcher.FetchNationalDeltaPage(ctx, planit.NationalDeltaQuery{
		DifferentStart: maskCutoff,
		Mask:           h.opts.Mask,
		MaskCutoff:     maskCutoff,
		StartIndex:     0,
	})
	if ferr != nil {
		var rl *planit.RateLimitError
		if errors.As(ferr, &rl) {
			out.rateLimited = true
			out.retryAfter = rl.RetryAfter
		} else {
			out.err = ferr
			out.timedOut = isTimeoutError(ferr)
		}
		// out.watermarkAfter is still the zero time here — the seeding fetch
		// itself failed, so nothing was ever established this run. Recording
		// an epoch-to-now "age" would mislead rather than signal genuine
		// staleness, so recordRunMetrics's own IsZero guard skips that call;
		// the rate-limit counters still fire when applicable.
		h.recordRunMetrics(ctx, out, now)
		h.setSpanAttributes(span, out, time.Time{}, maskCutoff, true)
		return out
	}

	out.pages = 1
	out.planitTotal = res.Total
	out.recordsSeen = len(res.Applications)

	head := time.Time{}
	for _, app := range res.Applications {
		if app.LastDifferent.After(head) {
			head = app.LastDifferent
		}
	}
	if head.IsZero() {
		head = now
	}
	out.watermarkAfter = head

	if serr := h.watermark.save(ctx, now, head, nil); serr != nil {
		out.err = serr
	}

	h.recordRunMetrics(ctx, out, now)
	h.setSpanAttributes(span, out, time.Time{}, maskCutoff, true)
	return out
}

// recordRunMetrics records one RunOnePage or seed invocation's lane-tagged
// business KPIs: applications ingested, the lane's watermark staleness, and
// — on a 429 — the rate-limit counter and Retry-After value (mirroring
// PollPlanItHandler.recordRetryAfter's header-present/absent branching).
//
// OldestHighWaterMarkAge repurposes the per-authority staleness gauge as
// "how far this lane's single global watermark trails now" — the correct
// analogue for a cursor-less national lane. It is skipped whenever
// out.watermarkAfter is still the zero time: that only happens when this
// run (or seed) bailed out before establishing any watermark at all (a
// watermark-read failure, or a failed/rate-limited seed fetch), and an
// epoch-to-now "age" in that case would mislead rather than signal
// staleness.
func (h *NationalLaneHandler) recordRunMetrics(ctx context.Context, out laneOutcome, now time.Time) {
	rec := h.recorder()
	lane := string(h.opts.Lane)

	rec.ApplicationsIngested(ctx, out.recordsIngested, lane)
	if !out.watermarkAfter.IsZero() {
		rec.OldestHighWaterMarkAge(ctx, now.Sub(out.watermarkAfter).Seconds(), lane, sentinelIDForLane(h.opts.Lane), out.watermarkAfter.IsZero())
	}
	if out.rateLimited {
		rec.RateLimited(ctx, lane)
		h.recordRetryAfter(ctx, rec, out.retryAfter, lane)
	}
}

// recordRetryAfter records the parsed Retry-After value (seconds) for a
// lane's 429, tagging header_present so dashboards distinguish a PlanIt 429
// with no Retry-After header (value 0, header_present=false) from a real
// small backoff — mirroring PollPlanItHandler.recordRetryAfter.
func (h *NationalLaneHandler) recordRetryAfter(ctx context.Context, rec metricsRecorder, retryAfter *time.Duration, lane string) {
	sentinelID := sentinelIDForLane(h.opts.Lane)
	if retryAfter == nil {
		rec.RetryAfterSeconds(ctx, 0, lane, sentinelID, false)
		return
	}
	rec.RetryAfterSeconds(ctx, retryAfter.Seconds(), lane, sentinelID, true)
}

// setSpanAttributes stamps the "PlanIt national lane poll" span with the full
// telemetry set — the safety mechanism a silent-skip bug would otherwise
// defeat with no error, no 429, and no alert. differentStart is the
// different_start prefilter actually sent this run (the mask cutoff on a
// seeding run, the watermark's calendar date on a normal walk), so spans can
// be grouped by that day to check the records_seen == planit.total invariant.
// seeded tags a first-run seed, so a seeding run's recordsIngested==0 is never
// misread as a stall.
func (h *NationalLaneHandler) setSpanAttributes(span trace.Span, out laneOutcome, watermarkBefore, differentStart time.Time, seeded bool) {
	attrs := []attribute.KeyValue{
		attribute.String("poll.lane", string(h.opts.Lane)),
		attribute.Int("poll.records_seen", out.recordsSeen),
		attribute.Int("poll.records_ingested", out.recordsIngested),
		attribute.Int("poll.pages", out.pages),
		attribute.String("poll.watermark_before", formatWatermark(watermarkBefore)),
		attribute.String("poll.watermark_after", formatWatermark(out.watermarkAfter)),
		attribute.Bool("poll.rate_limited", out.rateLimited),
		attribute.Bool("poll.cap_hit", out.capHit),
		attribute.Bool("poll.seeded", seeded),
		attribute.String("poll.different_start", differentStart.UTC().Format("2006-01-02")),
	}
	if out.planitTotal != nil {
		attrs = append(attrs, attribute.Int("planit.total", *out.planitTotal))
	}
	if out.err != nil {
		attrs = append(attrs, attribute.String("poll.error", out.err.Error()))
	}
	span.SetAttributes(attrs...)
}

// formatWatermark renders a lane watermark as RFC3339, or "" for a lane that
// has never ingested anything (the zero time.Time).
func formatWatermark(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}

// nextWorkPlanner is the consumer-side slice of *Planner NationalPollHandler
// needs. Declared here (not planner.go) per the consumer-side-interface
// convention.
type nextWorkPlanner interface {
	NextWork(state PlannerState, now time.Time) *WorkItem
}

// NationalPollOptions tune NationalPollHandler.Handle's planner/executor
// loop (ADR 0044).
type NationalPollOptions struct {
	// HandlerBudget is the soft wall-clock deadline Handle stops starting new
	// work before (mirrors HandlerOptions.HandlerBudget on the legacy
	// per-authority handler.go). Zero disables it (never time-bounded by
	// budget alone — only by the planner returning nil, an error, or a 429).
	HandlerBudget time.Duration
}

// NationalPollHandler runs the ADR 0044 resumable, checkpointed poll cycle:
// a planner/executor loop across four lanes (A/B always eligible, C
// daytime-only, D out-of-hours), one page per iteration, checkpointed after
// every page. It satisfies the Orchestrator's cycleHandler interface, so it
// plugs into the existing Service-Bus-triggered orchestrator (orchestrator.go,
// ADR 0024) unchanged — the trigger/lease machinery does not know or care
// which concrete handler it drives.
type NationalPollHandler struct {
	laneA   *NationalLaneHandler
	laneB   *NationalLaneHandler
	laneC   *InverseMaskLaneHandler // nil skips Lane C entirely (e.g. a test exercising only the critical path)
	laneD   *BackfillHandler        // nil skips Lane D (GH#967, ADR 0042) — the shape until POLLING_BACKFILL_ENABLED flips on
	planner nextWorkPlanner
	opts    NationalPollOptions
	flusher pushFlusher
	now     func() time.Time
	logger  *slog.Logger

	// metrics records towncrier.polling.cycles_completed for this handler's
	// cycles, wired via WithMetrics, mirroring PollPlanItHandler.metrics. nil
	// until wired (the no-metrics default).
	metrics metricsRecorder
}

// NewNationalPollHandler wires the handler over its planner and Lane A/B/C.
// laneC may be nil to skip Lane C entirely.
func NewNationalPollHandler(
	laneA, laneB *NationalLaneHandler,
	laneC *InverseMaskLaneHandler,
	planner nextWorkPlanner,
	opts NationalPollOptions,
	now func() time.Time,
	logger *slog.Logger,
) *NationalPollHandler {
	return &NationalPollHandler{laneA: laneA, laneB: laneB, laneC: laneC, planner: planner, opts: opts, now: now, logger: logger}
}

// WithBackfill wires Lane D (GH#967, ADR 0042), the paced historical backfill
// lane. nil is the safe default — loadPlannerState/execOnePage's nil guards
// skip it entirely — so cmd/worker's buildPollOrchestrator can call this
// unconditionally with whatever POLLING_BACKFILL_ENABLED produced (a real
// *BackfillHandler, or nil) without a separate branch. Returns the handler
// for chaining.
func (h *NationalPollHandler) WithBackfill(laneD *BackfillHandler) *NationalPollHandler {
	h.laneD = laneD
	return h
}

// WithPushFlusher wires the poll-cycle push coalescer (GH#784), mirroring
// PollPlanItHandler.WithPushFlusher. Returns the handler for chaining.
func (h *NationalPollHandler) WithPushFlusher(f pushFlusher) *NationalPollHandler {
	h.flusher = f
	return h
}

// WithMetrics wires the metrics recorder this handler records
// towncrier.polling.cycles_completed on, mirroring
// PollPlanItHandler.WithMetrics. A post-construction setter, so tests that
// don't supply one are unaffected; cmd/worker calls it once after
// construction. Returns the handler for chaining.
func (h *NationalPollHandler) WithMetrics(rec metricsRecorder) *NationalPollHandler {
	h.metrics = rec
	return h
}

// recorder returns a non-nil recorder so call sites can record
// unconditionally, mirroring PollPlanItHandler.recorder.
func (h *NationalPollHandler) recorder() metricsRecorder {
	if h.metrics == nil {
		return noopMetrics{}
	}
	return h.metrics
}

// commonLaneOutcome unifies the different lane executors' outcome shapes
// (laneOutcome for A/B/C, backfillOutcome for D) into the handful of fields
// Handle's loop needs to decide what to do next.
type commonLaneOutcome struct {
	recordsIngested int
	rateLimited     bool
	retryAfter      *time.Duration
	err             error
	// timedOut mirrors laneOutcome.timedOut (Lane D's backfillOutcome carries
	// no equivalent, so it always reads false from that branch — Lane D is
	// out of scope for tc-pmh5y's timeout classification).
	timedOut bool
}

// Handle runs one ADR 0044 poll cycle: a planner/executor loop replacing the
// old fixed A -> B -> (C) -> D sequence (ADR 0041/0042):
//
//	for {
//	    if budget exhausted:  TimeBounded; stop
//	    item := planner.NextWork(state, now)   // pure, no I/O
//	    if item == nil:       Natural; stop
//	    out := execOnePage(item.Lane)          // ONE fetch + ingest + checkpoint
//	    if out.rateLimited:   RateLimited; stop
//	    if out.err:           log it; stop (safe stop, last checkpoint holds)
//	}
//
// The loop breaks on the FIRST 429 or error from ANY lane — there is no
// per-lane backoff fold to forget, unlike the old outC-dropped bug
// (tc-mc0hf): a single `break` covers every lane uniformly. Exactly one
// ComputeNextRun/PublishAt happens per Handle call, via the unchanged
// orchestrator (ADR 0024) — the loop is a code boundary inside one process,
// one lease, one trigger, one budget, never a second job or trigger chain.
func (h *NationalPollHandler) Handle(ctx context.Context) (PollPlanItResult, error) {
	if h.flusher != nil {
		h.flusher.Reset()
	}

	var deadline time.Time
	hasDeadline := h.opts.HandlerBudget > 0
	if hasDeadline {
		deadline = h.now().UTC().Add(h.opts.HandlerBudget)
	}
	budgetExhausted := func() bool {
		return hasDeadline && !h.now().UTC().Before(deadline)
	}

	var (
		totalIngested int
		lastErr       error
		rateLimited   bool
		retryAfter    *time.Duration
		reason        = TerminationNatural
		pagesRun      = map[LaneName]int{}
	)

loop:
	for {
		if ctx.Err() != nil || budgetExhausted() {
			reason = TerminationTimeBounded
			break
		}

		state, err := h.loadPlannerState(ctx, pagesRun)
		if err != nil {
			h.logger.ErrorContext(ctx, "poll cycle: load planner state failed", "error", err)
			lastErr = err
			break
		}

		item := h.planner.NextWork(state, h.now().UTC())
		if item == nil {
			reason = TerminationNatural
			break
		}

		out := h.execOnePage(ctx, item.Lane)
		pagesRun[item.Lane]++
		totalIngested += out.recordsIngested

		if out.err != nil {
			lastErr = out.err
			if out.timedOut {
				reason = TerminationTimeout
			}
			break loop
		}
		if out.rateLimited {
			rateLimited = true
			retryAfter = out.retryAfter
			reason = TerminationRateLimited
			break loop
		}
	}

	if h.flusher != nil {
		if err := h.flusher.Flush(ctx); err != nil {
			h.logger.ErrorContext(ctx, "push flush failed", "error", err)
		}
	}

	laneErrors := 0
	if lastErr != nil {
		laneErrors = 1
	}

	h.recorder().CycleCompleted(ctx, "National", reason.TelemetryValue())

	return PollPlanItResult{
		ApplicationCount:  totalIngested,
		AuthoritiesPolled: 0, // no per-authority concept in the national lanes
		RateLimited:       rateLimited,
		TerminationReason: reason,
		AuthorityErrors:   laneErrors,
		RetryAfter:        retryAfter,
		CycleType:         "National",
	}, nil
}

// loadPlannerState loads fresh PlannerState from the stores: a handful of
// cheap point-reads (one per wired lane's sentinel row, plus Lane D's
// singleton backfill state) — there are only 4 lanes, not 485 authorities,
// so this is called once per loop iteration rather than once per Handle
// call, keeping the planner's view of the world always current without any
// manual in-memory bookkeeping to keep in sync.
//
// pagesRun is this Handle call's own in-memory per-lane page counter
// (reset every call, never persisted): a lane with a configured MaxPages
// that has already run that many pages THIS cycle is reported to the
// planner as if it had just run and had no active cursor — i.e. excluded
// from candidacy for the rest of this cycle — without touching its real
// persisted state at all, so the walk resumes exactly where it left off
// next cycle (see NationalLaneOptions.MaxPages).
func (h *NationalPollHandler) loadPlannerState(ctx context.Context, pagesRun map[LaneName]int) (PlannerState, error) {
	var st PlannerState
	now := h.now().UTC()

	_, aLast, aCursor, err := h.laneA.watermark.get(ctx)
	if err != nil {
		return st, fmt.Errorf("load lane A state: %w", err)
	}
	st.LaneA = LaneState{LastPollTime: aLast, Cursor: aCursor}

	_, bLast, bCursor, err := h.laneB.watermark.get(ctx)
	if err != nil {
		return st, fmt.Errorf("load lane B state: %w", err)
	}
	st.LaneB = LaneState{LastPollTime: bLast, Cursor: bCursor}

	if h.laneC != nil {
		_, cLast, cCursor, err := h.laneC.watermark.get(ctx)
		if err != nil {
			return st, fmt.Errorf("load lane C state: %w", err)
		}
		st.LaneC = &LaneState{LastPollTime: cLast, Cursor: cCursor}
	}

	if h.laneD != nil {
		bs, err := h.laneD.state.Get(ctx)
		if err != nil {
			return st, fmt.Errorf("load lane D state: %w", err)
		}
		st.LaneD = &LaneDState{LastPollTime: bs.LastRunTime, Complete: bs.Complete}
	}

	if maxPages := h.laneA.opts.MaxPages; maxPages != nil && pagesRun[LaneA] >= *maxPages {
		st.LaneA = LaneState{LastPollTime: now}
	}
	if maxPages := h.laneB.opts.MaxPages; maxPages != nil && pagesRun[LaneB] >= *maxPages {
		st.LaneB = LaneState{LastPollTime: now}
	}

	return st, nil
}

// execOnePage dispatches to the named lane's one-page executor and maps its
// outcome onto the shape Handle's loop needs. Lane D's "one page" is its own
// existing Run (unchanged, ADR 0042) — it already bounds and checkpoints
// itself internally (up to MaxPagesPerCycle pages, persisting after each) —
// so calling it once is "one unit of work" from Handle's perspective,
// exactly like every other lane's single page. Lane D deliberately never
// contributes to ApplicationCount: it is a data-quality/coverage lane, not
// part of the notification-bearing critical path (its Ingester is built
// with nil decision/enqueuer collaborators and structurally cannot notify).
func (h *NationalPollHandler) execOnePage(ctx context.Context, lane LaneName) commonLaneOutcome {
	switch lane {
	case LaneA:
		out := h.laneA.RunOnePage(ctx)
		if out.err != nil {
			h.logger.ErrorContext(ctx, "lane A poll error", "error", out.err)
		}
		return commonLaneOutcome{recordsIngested: out.recordsIngested, rateLimited: out.rateLimited, retryAfter: out.retryAfter, err: out.err, timedOut: out.timedOut}
	case LaneB:
		out := h.laneB.RunOnePage(ctx)
		if out.err != nil {
			h.logger.ErrorContext(ctx, "lane B poll error", "error", out.err)
		}
		return commonLaneOutcome{recordsIngested: out.recordsIngested, rateLimited: out.rateLimited, retryAfter: out.retryAfter, err: out.err, timedOut: out.timedOut}
	case LaneC:
		if h.laneC == nil {
			return commonLaneOutcome{}
		}
		out := h.laneC.RunOnePage(ctx)
		if out.err != nil {
			h.logger.ErrorContext(ctx, "lane C poll error", "error", out.err)
		}
		return commonLaneOutcome{recordsIngested: out.recordsIngested, rateLimited: out.rateLimited, retryAfter: out.retryAfter, err: out.err, timedOut: out.timedOut}
	case LaneD:
		if h.laneD == nil {
			return commonLaneOutcome{}
		}
		out := h.laneD.Run(ctx)
		if out.err != nil {
			h.logger.ErrorContext(ctx, "lane D backfill error", "error", out.err)
		}
		return commonLaneOutcome{rateLimited: out.rateLimited, retryAfter: out.retryAfter, err: out.err}
	default:
		return commonLaneOutcome{}
	}
}
