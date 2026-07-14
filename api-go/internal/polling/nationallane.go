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
// lane's last-run time, and the cursor columns are always nil — lanes A/B/C
// are cursor-less by design (ADR 0041: "a single timestamp, not 485
// cursors"). It is a thin wrapper over the existing pollStateAccess
// Get/Save — no new store, no new columns.
type laneWatermarkStore struct {
	state      pollStateAccess
	sentinelID int
}

func newLaneWatermarkStore(state pollStateAccess, sentinelID int) *laneWatermarkStore {
	return &laneWatermarkStore{state: state, sentinelID: sentinelID}
}

// get returns the lane's persisted watermark and last-run time, both zero when
// the lane has never run (no sentinel row yet).
func (s *laneWatermarkStore) get(ctx context.Context) (watermark, lastRun time.Time, err error) {
	st, found, err := s.state.Get(ctx, s.sentinelID)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	if !found {
		return time.Time{}, time.Time{}, nil
	}
	return st.HighWaterMark, st.LastPollTime, nil
}

// save persists the lane's watermark and this run's timestamp, with no
// cursor (lanes A/B/C never use one).
func (s *laneWatermarkStore) save(ctx context.Context, lastRun, watermark time.Time) error {
	return s.state.Save(ctx, s.sentinelID, lastRun, watermark, nil)
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
func (h *NationalLaneHandler) Run(ctx context.Context) laneOutcome {
	ctx, span := otel.Tracer(tracerName).Start(ctx, "PlanIt national lane poll")
	defer span.End()

	now := h.now().UTC()
	var out laneOutcome

	watermarkBefore, _, err := h.watermark.get(ctx)
	if err != nil {
		out.err = fmt.Errorf("lane %s: read watermark: %w", h.opts.Lane, err)
		span.SetAttributes(attribute.String("poll.lane", string(h.opts.Lane)))
		return out
	}
	out.watermarkBefore = watermarkBefore

	maskCutoff := truncateToDate(now.Add(-h.opts.MaskWindow))
	// On a lane's first-ever run there is no watermark to prefilter on; the
	// mask cutoff itself is the widest sane different_start, so the first run
	// behaves like a bounded national sweep of the mask window rather than an
	// unprefiltered query (never sent — ADR 0041's 11.7s-vs-0.2s guardrail).
	prefilterDate := maskCutoff
	if !watermarkBefore.IsZero() {
		prefilterDate = truncateToDate(watermarkBefore)
	}

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

	if serr := h.watermark.save(ctx, now, newWatermark); serr != nil && out.err == nil {
		out.err = serr
	}

	attrs := []attribute.KeyValue{
		attribute.String("poll.lane", string(h.opts.Lane)),
		attribute.Int("poll.records_seen", out.recordsSeen),
		attribute.Int("poll.records_ingested", out.recordsIngested),
		attribute.Int("poll.pages", out.pages),
		attribute.String("poll.watermark_before", formatWatermark(watermarkBefore)),
		attribute.String("poll.watermark_after", formatWatermark(out.watermarkAfter)),
		attribute.Bool("poll.rate_limited", out.rateLimited),
		attribute.Bool("poll.cap_hit", out.capHit),
	}
	if out.planitTotal != nil {
		attrs = append(attrs, attribute.Int("planit.total", *out.planitTotal))
	}
	if out.err != nil {
		attrs = append(attrs, attribute.String("poll.error", out.err.Error()))
	}
	span.SetAttributes(attrs...)

	return out
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
	flusher pushFlusher
	now     func() time.Time
	logger  *slog.Logger
}

// NewNationalPollHandler wires the three lanes. laneC may be nil to skip
// reconciliation.
func NewNationalPollHandler(laneA, laneB *NationalLaneHandler, laneC *ReconciliationHandler, now func() time.Time, logger *slog.Logger) *NationalPollHandler {
	return &NationalPollHandler{laneA: laneA, laneB: laneB, laneC: laneC, now: now, logger: logger}
}

// WithPushFlusher wires the poll-cycle push coalescer (GH#784), mirroring
// PollPlanItHandler.WithPushFlusher. Returns the handler for chaining.
func (h *NationalPollHandler) WithPushFlusher(f pushFlusher) *NationalPollHandler {
	h.flusher = f
	return h
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

	rateLimited := outA.rateLimited || outB.rateLimited
	var retryAfter *time.Duration
	switch {
	case outA.retryAfter != nil:
		retryAfter = outA.retryAfter
	case outB.retryAfter != nil:
		retryAfter = outB.retryAfter
	}

	reason := TerminationNatural
	switch {
	case rateLimited:
		reason = TerminationRateLimited
	case outA.capHit || outB.capHit:
		reason = TerminationTimeBounded
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
