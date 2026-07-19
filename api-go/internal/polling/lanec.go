// ADR 0044: Lane C, the national inverse-mask reconciliation lane. Replaces
// the per-authority ReconciliationHandler (deleted, tc-mc0hf's breaker along
// with it) with a single national query, walked ASCENDING over a pinned,
// contiguously-tiled epoch on last_different — the resumable core of ADR
// 0044's fix for the per-authority sweep's 429 collisions (485 requests plus
// hydration fan-out per pass) and the general stateless-re-walk livelock.
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

// inverseMaskFetcher is the consumer-side slice of the PlanIt client Lane C
// needs: one ascending epoch page, and a full-record hydration fetch by uid.
// *planit.Client satisfies both.
type inverseMaskFetcher interface {
	FetchInverseMaskPage(ctx context.Context, q planit.NationalInverseMaskQuery) (planit.FetchPageResult, error)
	FetchByUID(ctx context.Context, uid string) (planit.FetchPageResult, error)
}

// InverseMaskOptions tune Lane C's mask cutoff (ADR 0044).
type InverseMaskOptions struct {
	// MaskWindow is Lane A's start_date mask width. Lane C's end_date bound
	// is the same cutoff, inverted (today - MaskWindow), so the two lanes
	// partition the national change axis with no gap or overlap. A config
	// dial (POLLING_LANE_A_MASK_DAYS), not a correctness boundary owned by
	// this lane.
	MaskWindow time.Duration
}

// InverseMaskLaneHandler runs ADR 0044's Lane C: one page per call of a
// national, ascending, epoch-bounded inverse-mask query — the complement of
// Lane A/B's masked-delta band, reconciling old applications' status drift
// the delta axis structurally cannot see. Diffs each light row against
// Postgres on app_state and decided_date only (last_different is DROPPED
// from the diff — PlanIt bumps it on every re-index, so keeping it would
// flag every churned old record as a straggler, the old per-authority
// lane's measured hydration-amplification bug) and hydrates only genuine
// changes.
type InverseMaskLaneHandler struct {
	fetcher   inverseMaskFetcher
	watermark *laneWatermarkStore
	ingester  *Ingester
	apps      applicationStore
	opts      InverseMaskOptions
	now       func() time.Time
	logger    *slog.Logger

	metrics metricsRecorder
}

// NewInverseMaskLaneHandler wires Lane C. now is injected so tests pin the
// clock.
func NewInverseMaskLaneHandler(
	fetcher inverseMaskFetcher,
	state pollStateAccess,
	apps applicationStore,
	opts InverseMaskOptions,
	now func() time.Time,
	logger *slog.Logger,
) *InverseMaskLaneHandler {
	return &InverseMaskLaneHandler{
		fetcher:   fetcher,
		watermark: newLaneWatermarkStore(state, sentinelLaneC),
		ingester:  NewIngester(apps, nil, nil),
		apps:      apps,
		opts:      opts,
		now:       now,
		logger:    logger,
	}
}

// WithFanOut wires the notification fan-out collaborators onto Lane C's
// hydration ingests, mirroring the other lanes' WithFanOut (including the
// nil-ingester guard, so calling this on a zero-value InverseMaskLaneHandler
// never panics). Returns the handler for chaining.
func (h *InverseMaskLaneHandler) WithFanOut(decision DecisionDispatcher, enqueuer NotificationEnqueuer) *InverseMaskLaneHandler {
	if h.ingester == nil {
		h.ingester = &Ingester{}
	}
	h.ingester.decision = decision
	h.ingester.enqueuer = enqueuer
	return h
}

// WithMetrics wires the metrics recorder Lane C records its per-page
// ApplicationsIngested count on, mirroring the other lanes' WithMetrics.
// Returns the handler for chaining.
func (h *InverseMaskLaneHandler) WithMetrics(rec metricsRecorder) *InverseMaskLaneHandler {
	h.metrics = rec
	return h
}

// recorder returns a non-nil recorder so call sites can record
// unconditionally, mirroring the other lanes' recorder.
func (h *InverseMaskLaneHandler) recorder() metricsRecorder {
	if h.metrics == nil {
		return noopMetrics{}
	}
	return h.metrics
}

// RunOnePage executes exactly one page of Lane C's ascending epoch walk (ADR
// 0044 §5): anchor a new epoch when none is active, or resume the active one
// at its persisted NextIndex; fetch one page; diff/hydrate genuinely changed
// rows; checkpoint. The sentinel row's HighWaterMark holds the pinned
// epoch_upper, Cursor.DifferentStart doubles as epoch_lower, and
// Cursor.NextIndex is the ascending record offset — the existing PollCursor
// shape, reused with no schema migration.
func (h *InverseMaskLaneHandler) RunOnePage(ctx context.Context) laneOutcome {
	ctx, span := otel.Tracer(tracerName).Start(ctx, "PlanIt Lane C inverse-mask poll")
	defer span.End()

	now := h.now().UTC()
	var out laneOutcome

	epochUpper, _, cursor, err := h.watermark.get(ctx)
	if err != nil {
		out.err = fmt.Errorf("lane C: read epoch state: %w", err)
		span.SetAttributes(attribute.String("poll.lane", string(LaneC)))
		return out
	}

	maskCutoff := truncateToDate(now.Add(-h.opts.MaskWindow))

	if epochUpper.IsZero() {
		// Never run: seed like Lane A/B, but Lane C's epoch is purely
		// time-bound (no "head record" to discover), so seeding needs NO
		// PlanIt request at all: anchor a zero-width epoch (epoch_lower ==
		// epoch_upper == now). Nothing can satisfy last_different >
		// epoch_lower when epoch_lower == epoch_upper == now, so a fetch
		// here would be knowably wasted; the NEXT call anchors the first
		// real epoch from this seeded ceiling. Forward-flow only — never a
		// one-time replay of Lane C's entire historical inverse-mask corpus
		// (which would be exactly the red-line full-window re-scan ADR
		// 0041/0044 reject, just spread over many cycles instead of one).
		if serr := h.watermark.save(ctx, now, now, nil); serr != nil {
			out.err = serr
		}
		out.watermarkAfter = now
		h.recordOutcome(ctx, out)
		h.setSpanAttributes(span, out, time.Time{}, truncateToDate(now), true)
		return out
	}

	var epochLower time.Time
	if cursor != nil {
		// Resume the active epoch at its persisted position.
		epochLower = cursor.DifferentStart
	} else {
		// No active cursor: anchor a fresh epoch. The just-drained epoch's
		// ceiling becomes this epoch's floor (contiguous tiling, ADR 0044
		// §5 — no gap, a stall just widens the next window) and a new
		// ceiling pins at now.
		epochLower = epochUpper
		epochUpper = now
	}
	out.watermarkBefore = epochLower

	startIndex := 0
	if cursor != nil {
		startIndex = cursor.NextIndex
	}

	differentStart := truncateToDate(epochLower)
	res, ferr := h.fetcher.FetchInverseMaskPage(ctx, planit.NationalInverseMaskQuery{
		EpochLower: differentStart,
		MaskCutoff: maskCutoff,
		StartIndex: startIndex,
	})
	if ferr != nil {
		var rl *planit.RateLimitError
		if errors.As(ferr, &rl) {
			out.rateLimited = true
			out.retryAfter = rl.RetryAfter
		} else {
			out.err = ferr
		}
		out.watermarkAfter = epochUpper
		h.recordOutcome(ctx, out)
		h.setSpanAttributes(span, out, epochLower, differentStart, false)
		return out
	}

	out.pages = 1
	out.planitTotal = res.Total
	out.recordsSeen = len(res.Applications)

	reachedCeiling := false
	stoppedEarly := false
	for _, light := range res.Applications {
		// Ascending walk, exact-instant skip: a record at or before
		// epochLower was already handled by the PREVIOUS epoch — the
		// different_start prefilter is only date-granular, so the boundary
		// day can re-serve records this epoch has no business re-touching.
		if !light.LastDifferent.After(epochLower) {
			continue
		}
		// Pinned ceiling: a record past epoch_upper belongs to a FUTURE
		// epoch (it changed again after this epoch anchored) — stop here,
		// mirroring NationalLaneHandler's descending reachedBoundary in the
		// opposite direction. Ascending order means every remaining record
		// on this page, and every later page, also exceeds the ceiling, so
		// the whole epoch is done, not just this page.
		if light.LastDifferent.After(epochUpper) {
			reachedCeiling = true
			break
		}
		if perr := h.processStraggler(ctx, light, &out); perr != nil {
			out.err = perr
			stoppedEarly = true
			break
		}
		if out.rateLimited {
			// A hydration 429 trips the SAME "stop everything" rule as a
			// page-fetch 429 (ADR 0044: one break on the first 429 from ANY
			// lane) — never follow a rejected request with more requests.
			stoppedEarly = true
			break
		}
	}

	if stoppedEarly {
		out.watermarkAfter = epochUpper
		h.recordOutcome(ctx, out)
		h.setSpanAttributes(span, out, epochLower, differentStart, false)
		return out
	}

	nextIndex := startIndex + len(res.Applications)
	epochDrained := reachedCeiling || !res.HasMorePages

	if epochDrained {
		// Epoch complete: pin the drained epoch's ceiling as HighWaterMark
		// (it becomes the next epoch's floor on the next anchor) and clear
		// the cursor.
		if serr := h.watermark.save(ctx, now, epochUpper, nil); serr != nil && out.err == nil {
			out.err = serr
		}
	} else {
		newCursor := &PollCursor{DifferentStart: epochLower, NextIndex: nextIndex, KnownTotal: res.Total}
		if serr := h.watermark.save(ctx, now, epochUpper, newCursor); serr != nil && out.err == nil {
			out.err = serr
		}
	}
	out.watermarkAfter = epochUpper

	h.recordOutcome(ctx, out)
	h.setSpanAttributes(span, out, epochLower, differentStart, false)
	return out
}

// processStraggler diffs one light inverse-mask row against Postgres on
// app_state and decided_date only (existence counts as a difference too) and
// hydrates when it genuinely differs. authorityCode is built from the light
// row's area_id (ADR 0044's national-query correctness fix — see
// planit.inverseMaskSelectFields): PlanIt's uid is only unique within one
// authority, so a national query cannot diff/hydrate by uid alone without an
// authority scope, or two authorities sharing a bare uid could
// cross-contaminate. Any local failure (the existence read, or a hydrated
// Ingest) is a hard stop — out.err set, mirroring Lane A/B's own "never
// silently skip, freeze and let the next call resume" behaviour — so it is
// surfaced as an error rather than logged-and-skipped, unlike the deleted
// per-authority ReconciliationHandler.
func (h *InverseMaskLaneHandler) processStraggler(ctx context.Context, light applications.PlanningApplication, out *laneOutcome) error {
	authorityCode := strconv.Itoa(light.AreaID)
	existing, found, gerr := h.apps.GetByUID(ctx, light.UID, authorityCode)
	if gerr != nil {
		return fmt.Errorf("lane C: read existing application %q (authority %s): %w", light.UID, authorityCode, gerr)
	}
	if found && !inverseMaskDiffers(existing, light) {
		return nil
	}
	return h.hydrate(ctx, light.UID, light.AreaID, out)
}

// hydrate fetches one straggler's full record by uid and feeds it through
// the standard Ingester (identical fan-out to Lane A/B). wantAreaID guards
// against PlanIt's id_match lookup crossing authorities (FetchByUID carries
// no auth param): only a hydrated record whose AreaID matches the light row
// that flagged it is ingested; any other match is treated as "no matching
// record" and logged, exactly like a genuine miss. A rate limit is recorded
// on out (the caller stops the whole page/epoch on it, same as a page-fetch
// 429); any other fetch error, or an Ingest failure, is a hard stop.
func (h *InverseMaskLaneHandler) hydrate(ctx context.Context, uid string, wantAreaID int, out *laneOutcome) error {
	full, err := h.fetcher.FetchByUID(ctx, uid)
	if err != nil {
		var rl *planit.RateLimitError
		if errors.As(err, &rl) {
			out.rateLimited = true
			out.retryAfter = rl.RetryAfter
			return nil
		}
		return fmt.Errorf("lane C: hydration fetch %q: %w", uid, err)
	}
	for _, app := range full.Applications {
		if app.UID != uid || app.AreaID != wantAreaID {
			continue
		}
		if ierr := h.ingester.Ingest(ctx, app); ierr != nil {
			return fmt.Errorf("lane C: hydrated ingest %q: %w", uid, ierr)
		}
		out.recordsIngested++
		return nil
	}
	h.logger.WarnContext(ctx, "lane C: hydration fetch returned no matching record", "uid", uid, "areaId", wantAreaID)
	return nil
}

// inverseMaskDiffers reports whether the light inverse-mask row's app_state
// or decided_date differs from the persisted application — Lane C's
// straggler test (ADR 0044 §4). last_different is DELIBERATELY excluded:
// PlanIt bumps it on every re-index, so comparing it would flag every
// churned old record as a straggler and hydrate it for nothing — the
// measured hydration-amplification bug the old per-authority lane hit. A
// last_different-only churned row must NOT hydrate.
func inverseMaskDiffers(existing, light applications.PlanningApplication) bool {
	if !eqOptionalString(existing.AppState, light.AppState) {
		return true
	}
	return !eqOptionalTime(existing.DecidedDate, light.DecidedDate)
}

// eqOptionalString reports whether two optional string pointers carry the
// same value (both nil counts as equal).
func eqOptionalString(a, b *string) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}

// eqOptionalTime reports whether two optional time pointers carry the same
// instant (both nil counts as equal).
func eqOptionalTime(a, b *time.Time) bool {
	if a == nil || b == nil {
		return a == b
	}
	return a.Equal(*b)
}

// recordOutcome records Lane C's per-page ApplicationsIngested count and,
// on a 429, the rate-limit counter and Retry-After value — mirroring the
// other lanes' recordRunMetrics. Lane C deliberately skips
// OldestHighWaterMarkAge: its "watermark" (epoch_upper) resets to ~now on
// every anchor, so an epoch-to-now age would always read ~0 and never
// signal genuine backlog depth — mirroring BackfillHandler's identical
// omission for Lane D and its stated rationale.
func (h *InverseMaskLaneHandler) recordOutcome(ctx context.Context, out laneOutcome) {
	rec := h.recorder()
	rec.ApplicationsIngested(ctx, out.recordsIngested, string(LaneC))
	if out.rateLimited {
		rec.RateLimited(ctx, string(LaneC))
		if out.retryAfter == nil {
			rec.RetryAfterSeconds(ctx, 0, string(LaneC), sentinelLaneC, false)
		} else {
			rec.RetryAfterSeconds(ctx, out.retryAfter.Seconds(), string(LaneC), sentinelLaneC, true)
		}
	}
}

// setSpanAttributes stamps the "PlanIt Lane C inverse-mask poll" span,
// mirroring the other lanes' setSpanAttributes. epochLower is the epoch
// floor in effect this call (the seed cutoff on a seeding run);
// differentStart is the different_start value actually sent to PlanIt (or,
// on a seed, the date that would be used going forward — no request is
// sent). seeded tags a first-run seed so its recordsIngested==0 is never
// misread as a stall.
func (h *InverseMaskLaneHandler) setSpanAttributes(span trace.Span, out laneOutcome, epochLower, differentStart time.Time, seeded bool) {
	attrs := []attribute.KeyValue{
		attribute.String("poll.lane", string(LaneC)),
		attribute.Int("poll.records_seen", out.recordsSeen),
		attribute.Int("poll.records_ingested", out.recordsIngested),
		attribute.Int("poll.pages", out.pages),
		attribute.String("poll.epoch_lower", formatWatermark(epochLower)),
		attribute.String("poll.epoch_upper", formatWatermark(out.watermarkAfter)),
		attribute.Bool("poll.rate_limited", out.rateLimited),
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
