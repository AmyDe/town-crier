// ADR 0041 / GH#962: the churn-masked national delta poll. This file replaces
// the per-authority drain (handler.go, left compiling but unwired — see
// cmd/worker/main.go) with a single national query per lane and ONE global
// watermark per lane, persisted in the EXISTING poll_state table via a
// reserved sentinel authority_id (no schema migration: rollback stays a pure
// image redeploy).
package polling

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"github.com/AmyDe/town-crier/api-go/internal/planit"
)

// Sentinel poll_state.authority_id values reserved for the three ADR 0041
// lanes' global watermark bookkeeping. Real PlanIt authority ids are always
// positive (authorities.Lookup never emits <= 0), so a sentinel row can never
// collide with a real authority's poll state.
const (
	sentinelLaneA = -1
	sentinelLaneB = -2
	sentinelLaneC = -3
)

// LaneName tags which ADR 0041 lane produced a span/result: "A" (new
// applications), "B" (decisions), or "C" (reconciliation).
type LaneName string

// LaneA, LaneB, and LaneC are the three ADR 0041 lanes.
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

// laneWatermarkStore persists ONE lane's global delta watermark in the
// existing poll_state table via a reserved sentinel authority_id (see the
// package doc above): HighWaterMark holds the watermark, LastPollTime the
// lane's last-run time. Lane A/B never pass a cursor (ADR 0041: "a single
// timestamp, not 485 cursors") — get/save simply thread it through so those
// callers can ignore it. Lane C is the one exception (tc-tuge8/GH#971): its
// ReconciliationHandler reuses the same cursor column to resume a
// multi-cycle authority-list sweep (see reconciliation.go). It is a thin
// wrapper over the existing pollStateAccess Get/Save — no new store, no new
// columns either way.
type laneWatermarkStore struct {
	state      pollStateAccess
	sentinelID int
}

func newLaneWatermarkStore(state pollStateAccess, sentinelID int) *laneWatermarkStore {
	return &laneWatermarkStore{state: state, sentinelID: sentinelID}
}

// get returns the lane's persisted watermark, last-run time, and cursor. All
// three are zero/nil when the lane has never run (no sentinel row yet). Lane
// A/B callers discard the cursor return value; only Lane C reads it.
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
// Lane A/B always pass a nil cursor (they never use one); Lane C passes its
// authority-list resume position.
func (s *laneWatermarkStore) save(ctx context.Context, lastRun, watermark time.Time, cursor *PollCursor) error {
	return s.state.Save(ctx, s.sentinelID, lastRun, watermark, cursor)
}

// NationalLaneOptions configure one of ADR 0041's national delta lanes (A or
// B — Lane C has its own ReconciliationOptions).
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
	// MaxPages caps the descending page walk. nil = unbounded (Lane A: the
	// national delta is measured at ~6 pages/day). Non-nil hard-caps the
	// walk (Lane B: 20 pages/run — decision volume is unmeasured
	// pre-cutover, and this cap must not be removed).
	MaxPages *int
}

// NationalLaneHandler runs one ADR 0041 national delta lane (A or B): a
// single national PlanIt query (no auth param), masked by start_date or
// decided_start to filter out re-index churn, paged descending by
// last_different until a record at or before the lane's watermark is
// reached. State is ONE global timestamp — no per-authority cursors, no LRU
// authority selection.
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

// laneOutcome is one national lane run's result, carried both into
// PollPlanItResult (by the caller) and onto the lane's telemetry span.
type laneOutcome struct {
	recordsSeen     int
	recordsIngested int
	pages           int
	rateLimited     bool
	retryAfter      *time.Duration
	err             error
	planitTotal     *int
	watermarkBefore time.Time
	watermarkAfter  time.Time
	capHit          bool
}

// Run walks one national delta lane's descending pages from PlanIt's newest
// record down to the lane's watermark (ADR 0041): the coarse different_start
// prefilter narrows PlanIt's scan to roughly the right window, the
// start_date/decided_start mask filters out re-index churn, and the
// descending sort plus this in-memory timestamp watermark give exact delta
// semantics with no cursor to persist.
//
// A record whose LastDifferent is at or before the watermark was already
// ingested by a prior run (the watermark is set to that run's max ingested
// value) — paging stops there without re-ingesting it. Every record strictly
// newer that shares its page with the boundary record has already been
// ingested earlier in the same page's iteration (descending order means it
// comes first), so nothing between the old watermark and the new one is
// dropped.
//
// The watermark advances ONLY on a completely clean run (no fetch error, no
// 429, no page-cap or context cut-off): only then is every record between the
// old watermark and the new one accounted for. Any early stop leaves the
// watermark untouched — the next run re-walks the same range from scratch,
// which is wasted request budget, not a silent skip (Ingester's
// HasSameBusinessFieldsAs gate makes the redundant re-ingests free), and is
// the safe failure direction the ADR calls out: "never advance a watermark
// past a page that errored."
//
// A lane with no prior watermark (never run) does NOT walk this way at all —
// see seed.
func (h *NationalLaneHandler) Run(ctx context.Context) laneOutcome {
	ctx, span := otel.Tracer(tracerName).Start(ctx, "PlanIt national lane poll")
	defer span.End()

	now := h.now().UTC()
	var out laneOutcome

	watermarkBefore, _, _, err := h.watermark.get(ctx)
	if err != nil {
		out.err = fmt.Errorf("lane %s: read watermark: %w", h.opts.Lane, err)
		span.SetAttributes(attribute.String("poll.lane", string(h.opts.Lane)))
		// out is not fully computed here — the watermark read itself failed,
		// so there is nothing yet to report ingested and no watermark to age
		// (out.watermarkAfter is still the zero time). No metrics recorded,
		// mirroring the absence of a setSpanAttributes call on this early
		// bail-out.
		return out
	}
	out.watermarkBefore = watermarkBefore

	maskCutoff := truncateToDate(now.Add(-h.opts.MaskWindow))

	// A lane with no prior watermark has never run. Walking the mask window
	// here — as a normal delta walk would, since "> zero time" is true for
	// every real record — would be a historical backfill (forbidden: ADR 0041
	// rule 2) and, for Lane A's unbounded page walk, a red-line request
	// spike that never even finishes inside the cycle budget: a budget
	// cut-off is never a "clean run", so the watermark could never be set,
	// and every subsequent cycle would re-attempt the same full-window walk
	// forever. Seed instead: read PlanIt's current head from a single page-0
	// fetch and persist it, ingesting nothing. The old drain already held us
	// at the head (prod baseline max last_different 2026-07-14 05:14:58Z);
	// Lane C's reconciliation sweep is the backstop for the small forward-flow
	// gap a seed (rather than a backfill) leaves.
	if watermarkBefore.IsZero() {
		return h.seed(ctx, span, now, maskCutoff)
	}

	prefilterDate := truncateToDate(watermarkBefore)

	var (
		index       int
		maxIngested time.Time
	)

pageLoop:
	for {
		if h.opts.MaxPages != nil && out.pages >= *h.opts.MaxPages {
			out.capHit = true
			break
		}
		if ctx.Err() != nil {
			out.capHit = true
			break
		}

		res, ferr := h.fetcher.FetchNationalDeltaPage(ctx, planit.NationalDeltaQuery{
			DifferentStart: prefilterDate,
			Mask:           h.opts.Mask,
			MaskCutoff:     maskCutoff,
			StartIndex:     index,
		})
		if ferr != nil {
			var rl *planit.RateLimitError
			if errors.As(ferr, &rl) {
				out.rateLimited = true
				out.retryAfter = rl.RetryAfter
			} else {
				out.err = ferr
			}
			break pageLoop
		}

		out.pages++
		if out.pages == 1 {
			out.planitTotal = res.Total
		}
		out.recordsSeen += len(res.Applications)

		reachedBoundary := false
		for _, app := range res.Applications {
			if !app.LastDifferent.After(watermarkBefore) {
				reachedBoundary = true
				break
			}
			if ierr := h.ingester.Ingest(ctx, app); ierr != nil {
				out.err = ierr
				break pageLoop
			}
			out.recordsIngested++
			if app.LastDifferent.After(maxIngested) {
				maxIngested = app.LastDifferent
			}
		}
		if reachedBoundary {
			break
		}

		index += len(res.Applications)
		if !res.HasMorePages {
			break
		}
	}

	naturalEnd := out.err == nil && !out.rateLimited && !out.capHit
	newWatermark := watermarkBefore
	if naturalEnd && !maxIngested.IsZero() {
		newWatermark = maxIngested
	}
	out.watermarkAfter = newWatermark

	if serr := h.watermark.save(ctx, now, newWatermark, nil); serr != nil && out.err == nil {
		out.err = serr
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

// recordRunMetrics records one Run or seed invocation's lane-tagged business
// KPIs: applications ingested, the lane's watermark staleness, and — on a
// 429 — the rate-limit counter and Retry-After value (mirroring
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
// ADR 0041 telemetry set — the safety mechanism a silent-skip bug would
// otherwise defeat with no error, no 429, and no alert. differentStart is the
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

// NationalPollHandler runs ADR 0041's churn-masked national delta poll: Lane A
// (new applications) and Lane B (decisions) every cycle, plus Lane C
// (reconciliation) whenever its own, much longer interval has elapsed. It
// satisfies the Orchestrator's cycleHandler interface, so it plugs into the
// existing Service-Bus-triggered orchestrator (orchestrator.go, ADR 0024)
// unchanged — the trigger/lease machinery does not know or care which
// concrete handler it drives.
type NationalPollHandler struct {
	laneA   *NationalLaneHandler
	laneB   *NationalLaneHandler
	laneC   *ReconciliationHandler // nil skips Lane C entirely (e.g. a test exercising only the critical path)
	laneD   *BackfillHandler       // nil skips Lane D (GH#967, ADR 0042) — the shape until POLLING_BACKFILL_ENABLED flips on
	flusher pushFlusher
	now     func() time.Time
	logger  *slog.Logger

	// metrics records towncrier.polling.cycles_completed for this handler's
	// cycles, wired via WithMetrics, mirroring PollPlanItHandler.metrics. nil
	// until wired (the no-metrics default).
	metrics metricsRecorder
}

// NewNationalPollHandler wires the three lanes. laneC may be nil to skip
// reconciliation.
func NewNationalPollHandler(laneA, laneB *NationalLaneHandler, laneC *ReconciliationHandler, now func() time.Time, logger *slog.Logger) *NationalPollHandler {
	return &NationalPollHandler{laneA: laneA, laneB: laneB, laneC: laneC, now: now, logger: logger}
}

// WithBackfill wires Lane D (GH#967, ADR 0042), the paced historical backfill
// lane. nil is the safe default — Handle's nil guard skips it entirely — so
// cmd/worker's buildPollOrchestrator can call this unconditionally with
// whatever POLLING_BACKFILL_ENABLED produced (a real *BackfillHandler, or
// nil) without a separate branch. Returns the handler for chaining.
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

// Handle runs one national poll cycle: Lane A, then Lane B, then — only when
// due — Lane C. A lane error is logged and counted (AuthorityErrors) but
// never fails the cycle: there is no per-authority state to strand and no
// cursor to corrupt, so the next hourly run is self-healing. Handle returns a
// non-nil error only when it cannot even determine what to do (never reached
// today — kept for interface parity with cycleHandler, mirroring
// PollPlanItHandler.Handle's contract).
func (h *NationalPollHandler) Handle(ctx context.Context) (PollPlanItResult, error) {
	if h.flusher != nil {
		h.flusher.Reset()
	}

	outA := h.laneA.Run(ctx)
	outB := h.laneB.Run(ctx)

	if h.flusher != nil {
		if err := h.flusher.Flush(ctx); err != nil {
			h.logger.ErrorContext(ctx, "push flush failed", "error", err)
		}
	}

	if outA.err != nil {
		h.logger.ErrorContext(ctx, "lane A poll error", "error", outA.err)
	}
	if outB.err != nil {
		h.logger.ErrorContext(ctx, "lane B poll error", "error", outB.err)
	}

	laneErrors := 0
	if outA.err != nil {
		laneErrors++
	}
	if outB.err != nil {
		laneErrors++
	}

	if h.laneC != nil {
		now := h.now().UTC()
		if h.laneC.Due(ctx, now) {
			outC := h.laneC.Run(ctx)
			if outC.err != nil {
				h.logger.ErrorContext(ctx, "lane C reconciliation error", "error", outC.err)
			}
		}
	}

	// Lane D (GH#967, ADR 0042): runs unconditionally every cycle, nil-guarded
	// — unlike Lane C it is never Due-gated (there is no interval to check, it
	// always spends its small fixed page budget). It never contributes to
	// ApplicationCount/AuthorityErrors below: it is a data-quality/coverage
	// lane, not part of the critical path those fields describe, and it
	// structurally cannot notify (nil decision/enqueuer, no WithFanOut
	// method), so there is nothing here for it to affect. It DOES fold into
	// the rate-limit backoff below (rateLimited/retryAfter/reason, tc-hew73):
	// a 429 against Lane D specifically must still slow the next poll cycle,
	// exactly as an A/B 429 does, or PlanIt's Retry-After hint gets silently
	// dropped on the floor.
	var outD backfillOutcome
	if h.laneD != nil {
		outD = h.laneD.Run(ctx)
		if outD.err != nil {
			h.logger.ErrorContext(ctx, "lane D backfill error", "error", outD.err)
		}
	}

	rateLimited := outA.rateLimited || outB.rateLimited || outD.rateLimited
	var retryAfter *time.Duration
	switch {
	case outA.retryAfter != nil:
		retryAfter = outA.retryAfter
	case outB.retryAfter != nil:
		retryAfter = outB.retryAfter
	case outD.retryAfter != nil:
		retryAfter = outD.retryAfter
	}

	reason := TerminationNatural
	switch {
	case rateLimited:
		reason = TerminationRateLimited
	case outA.capHit || outB.capHit:
		reason = TerminationTimeBounded
	}

	h.recorder().CycleCompleted(ctx, "National", reason.TelemetryValue())

	return PollPlanItResult{
		ApplicationCount:  outA.recordsIngested + outB.recordsIngested,
		AuthoritiesPolled: 0, // no per-authority concept in the national lanes
		RateLimited:       rateLimited,
		TerminationReason: reason,
		AuthorityErrors:   laneErrors,
		RetryAfter:        retryAfter,
		CycleType:         "National",
	}, nil
}
