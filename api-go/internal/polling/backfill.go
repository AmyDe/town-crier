// GH#967 / ADR 0042: Lane D, the paced historical backfill lane. Where Lanes
// A/B/C (nationallane.go, reconciliation.go) walk PlanIt FORWARD from a
// watermark, Lane D creeps BACKWARD through PlanIt's national history one
// fixed-width date window at a time, feeding every record it sees through the
// existing, unmodified Ingester so GH#935's three-bucket classification does
// the enrichment (stale/NULL silent fields) and gap-fill (missing rows) work
// for free. It holds ONE piece of persisted state — not one row per authority
// — because it isn't checking any single authority's freshness, it is
// walking the whole national timeline once.
//
// The one invariant every other design choice here serves: a resident must
// never be notified about something found by looking backward. This lane's
// Ingester is constructed with nil decision/enqueuer collaborators
// (NewIngester(apps, nil, nil)), and — unlike NationalLaneHandler and
// ReconciliationHandler — BackfillHandler has NO WithFanOut method. There is
// no call cmd/worker's wiring could make, today or in a future edit, that
// attaches a notifier to this lane. That is a compile-time guarantee, not a
// runtime flag.
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

// backfillMetricsLane tags the backfill lane's metrics/telemetry, mirroring
// LaneA/LaneB/LaneC's single-letter tag convention without extending the ADR
// 0041 LaneName type: Lane D belongs to ADR 0042, a different design (no
// watermark, no per-authority sentinel row) that only shares the tag
// vocabulary for dashboard consistency.
const backfillMetricsLane = "D"

// backfillFetcher is the consumer-side slice of the PlanIt client Lane D
// needs: one page of the national, date-windowed backward sweep.
// *planit.Client satisfies it.
type backfillFetcher interface {
	FetchBackfillPage(ctx context.Context, windowStart, windowEnd time.Time, startIndex int) (planit.FetchPageResult, error)
}

// BackfillState is Lane D's ENTIRE persisted state: a singleton, not one row
// per authority. WindowEnd is the fixed upper start_date bound of the date
// window currently being drained (zero = never started). CursorNextIndex is
// how far pagination has progressed within that window. WindowRecordsSeen
// counts records seen so far in the window currently in progress, deciding
// whether a fully-drained window counts toward ConsecutiveEmptyWindows.
// Complete, once set, makes every subsequent Run a no-op forever — the lane
// has crept back far enough that consecutive windows have come back empty.
type BackfillState struct {
	WindowEnd               time.Time
	CursorNextIndex         int
	WindowRecordsSeen       int
	ConsecutiveEmptyWindows int
	Complete                bool
	LastRunTime             time.Time
}

// backfillStateAccess is the consumer-side slice of the backfill-state store
// BackfillHandler needs. Get/Save operate on the one singleton row — no
// candidate-id list, no LRU query, because there is exactly one thing to
// track.
type backfillStateAccess interface {
	Get(ctx context.Context) (BackfillState, error)
	Save(ctx context.Context, state BackfillState) error
}

// BackfillOptions tune Lane D's pace. WindowWidthDays is the width of each
// backward-sliding date window (default 90, mirrored from ADR 0041's mask
// width). MaxPagesPerCycle bounds how many pages Run fetches in a single
// call — small on purpose ("creep a little bit each hour", default 2).
// EmptyWindowsBeforeComplete is how many consecutive fully-drained,
// zero-record windows the lane tolerates before declaring itself done
// (default 12 ~= 3 years of national silence — conservative enough that it
// should never trip on real data, only on genuinely running out of history).
type BackfillOptions struct {
	WindowWidthDays            int
	MaxPagesPerCycle           int
	EmptyWindowsBeforeComplete int
}

// BackfillHandler runs ADR 0042's Lane D: a national, date-windowed backward
// sweep that enriches stale/NULL fields and fills coverage gaps via the
// existing Ingester, with no per-authority state and — structurally — no
// notification fan-out ever reachable from it.
type BackfillHandler struct {
	fetcher  backfillFetcher
	state    backfillStateAccess
	ingester *Ingester
	opts     BackfillOptions
	now      func() time.Time
	logger   *slog.Logger

	// metrics records towncrier.polling.applications_ingested (tagged "D") for
	// this lane's runs, wired via WithMetrics, mirroring the other lanes'
	// metrics field. nil until wired (the no-metrics default).
	metrics metricsRecorder
}

// NewBackfillHandler wires Lane D. now is injected so tests pin the clock.
// The ingester is ALWAYS constructed with nil decision/enqueuer collaborators
// — see the package doc above. There is no setter that can change that after
// construction.
func NewBackfillHandler(
	fetcher backfillFetcher,
	state backfillStateAccess,
	apps applicationStore,
	opts BackfillOptions,
	now func() time.Time,
	logger *slog.Logger,
) *BackfillHandler {
	return &BackfillHandler{
		fetcher:  fetcher,
		state:    state,
		ingester: NewIngester(apps, nil, nil), // notification safety: never wired, never wireable
		opts:     opts,
		now:      now,
		logger:   logger,
	}
}

// WithMetrics wires the metrics recorder this lane records its per-run
// applications-ingested count on, mirroring the other lanes' WithMetrics. A
// post-construction setter, so tests that don't supply one are unaffected;
// cmd/worker calls it once after construction (only when the lane is built at
// all — POLLING_BACKFILL_ENABLED gates that). Returns the handler for
// chaining.
func (h *BackfillHandler) WithMetrics(rec metricsRecorder) *BackfillHandler {
	h.metrics = rec
	return h
}

// recorder returns a non-nil recorder so call sites can record
// unconditionally, mirroring the other lanes' recorder.
func (h *BackfillHandler) recorder() metricsRecorder {
	if h.metrics == nil {
		return noopMetrics{}
	}
	return h.metrics
}

// backfillOutcome is one Run call's result, carried onto the telemetry span.
type backfillOutcome struct {
	pages           int
	recordsSeen     int
	recordsIngested int
	rateLimited     bool
	retryAfter      *time.Duration
	err             error
	planitTotal     *int
	complete        bool
}

// Run fetches up to MaxPagesPerCycle pages of the current backward-sliding
// window, ingesting every record through the existing Ingester (enrichment
// and gap-fill, for free, via GH#935's three-bucket classification). State is
// persisted after every successfully-processed page — a crash mid-cycle loses
// nothing, because re-fetching an already-ingested page is a free no-op under
// Ingest's HasSameBusinessFieldsAs/HasSameSilentFieldsAs gates.
//
// A fetch error or an ingest error stops the loop for this cycle WITHOUT
// persisting the failed page's progress — whatever prior pages in this same
// call succeeded stays persisted (mirrors NationalLaneHandler's "never
// advance past a page that errored").
//
// When a window is fully drained (HasMorePages goes false), the window slides
// back by WindowWidthDays unless EmptyWindowsBeforeComplete consecutive
// zero-record windows have now been seen, in which case the lane marks itself
// Complete and stops for good — every subsequent Run is then a no-op with no
// fetch at all.
func (h *BackfillHandler) Run(ctx context.Context) backfillOutcome {
	ctx, span := otel.Tracer(tracerName).Start(ctx, "PlanIt backfill sweep")
	defer span.End()

	now := h.now().UTC()
	var out backfillOutcome

	state, err := h.state.Get(ctx)
	if err != nil {
		out.err = fmt.Errorf("lane D: read backfill state: %w", err)
		return out
	}

	if state.Complete {
		out.complete = true
		span.SetAttributes(attribute.Bool("backfill.complete", true))
		return out
	}

	if state.WindowEnd.IsZero() {
		state.WindowEnd = truncateToDate(now)
		state.CursorNextIndex = 0
		state.WindowRecordsSeen = 0
	}

pageLoop:
	for out.pages < h.opts.MaxPagesPerCycle {
		if ctx.Err() != nil {
			break
		}

		windowStart := state.WindowEnd.AddDate(0, 0, -h.opts.WindowWidthDays)

		res, ferr := h.fetcher.FetchBackfillPage(ctx, windowStart, state.WindowEnd, state.CursorNextIndex)
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
		if out.planitTotal == nil {
			out.planitTotal = res.Total
		}
		out.recordsSeen += len(res.Applications)

		ingestFailed := false
		for _, app := range res.Applications {
			if ierr := h.ingester.Ingest(ctx, app); ierr != nil {
				out.err = ierr
				ingestFailed = true
				break
			}
			out.recordsIngested++
		}
		if ingestFailed {
			break pageLoop
		}

		state.CursorNextIndex += len(res.Applications)
		state.WindowRecordsSeen += len(res.Applications)
		state.LastRunTime = now

		if !res.HasMorePages {
			if state.WindowRecordsSeen == 0 {
				state.ConsecutiveEmptyWindows++
			} else {
				state.ConsecutiveEmptyWindows = 0
			}

			if state.ConsecutiveEmptyWindows >= h.opts.EmptyWindowsBeforeComplete {
				state.Complete = true
				out.complete = true
				if serr := h.state.Save(ctx, state); serr != nil && out.err == nil {
					out.err = serr
				}
				break pageLoop
			}

			// Window fully drained but the lane isn't done: slide back and keep
			// walking if this cycle's page budget allows.
			state.WindowEnd = windowStart
			state.CursorNextIndex = 0
			state.WindowRecordsSeen = 0
		}

		if serr := h.state.Save(ctx, state); serr != nil {
			out.err = serr
			break pageLoop
		}
	}

	h.recordRunMetrics(ctx, out)
	h.setSpanAttributes(span, out)
	return out
}

// recordRunMetrics records one Run invocation's applications-ingested count
// (tagged "D") and, on a 429, the rate-limit counter and Retry-After value —
// mirroring the other lanes' recordRunMetrics. Lane D has no forward
// watermark, so it does not record OldestHighWaterMarkAge: that gauge feeds
// the ADR 0041 freshness alert, and a backward-creeping window's "age" would
// be a meaningless, permanently-stale signal on that surface.
func (h *BackfillHandler) recordRunMetrics(ctx context.Context, out backfillOutcome) {
	rec := h.recorder()
	rec.ApplicationsIngested(ctx, out.recordsIngested, backfillMetricsLane)
	if out.rateLimited {
		rec.RateLimited(ctx, backfillMetricsLane)
		if out.retryAfter == nil {
			rec.RetryAfterSeconds(ctx, 0, backfillMetricsLane, 0, false)
		} else {
			rec.RetryAfterSeconds(ctx, out.retryAfter.Seconds(), backfillMetricsLane, 0, true)
		}
	}
}

// setSpanAttributes stamps the "PlanIt backfill sweep" span with Lane D's
// telemetry set, mirroring the other lanes' setSpanAttributes.
func (h *BackfillHandler) setSpanAttributes(span trace.Span, out backfillOutcome) {
	attrs := []attribute.KeyValue{
		attribute.Int("backfill.pages", out.pages),
		attribute.Int("backfill.records_seen", out.recordsSeen),
		attribute.Int("backfill.records_ingested", out.recordsIngested),
		attribute.Bool("backfill.complete", out.complete),
		attribute.Bool("backfill.rate_limited", out.rateLimited),
	}
	if out.planitTotal != nil {
		attrs = append(attrs, attribute.Int("planit.total", *out.planitTotal))
	}
	if out.err != nil {
		attrs = append(attrs, attribute.String("backfill.error", out.err.Error()))
	}
	span.SetAttributes(attrs...)
}
