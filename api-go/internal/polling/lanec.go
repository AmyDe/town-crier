// ADR 0044 §5 (as amended by #1127): Lane C, the national inverse-mask
// reconciliation lane. Replaces the per-authority ReconciliationHandler
// (deleted, tc-mc0hf's breaker along with it) with a single national query,
// walked ASCENDING over a bounded, rolling different=N window on
// last_different — the resumable core of ADR 0044's fix for the per-authority
// sweep's 429 collisions (485 requests plus hydration fan-out per pass) and
// the general stateless-re-walk livelock.
//
// #1127 replaced the original pinned-epoch / contiguous-tiling model: an
// absolute different_start floor is a coarse prefilter only (PlanIt has no
// different_end), so once the epoch cursor froze the floor stayed weeks in
// the past and every cycle asked PlanIt for a total+sort over every national
// record changed since — a query it could not answer inside its own 45s
// limit (bead tc-777e7, the 2026-07 → 2026-09 prod livelock). A rolling
// different=N window with N hard-capped small is inherently bounded: the
// query cost cannot grow without limit no matter how long the lane stalls.
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

// maxHydrationsPerPass bounds Lane C's straggler hydration fan-out within a
// single RunOnePage call (GH#986). Unlike Lane A/B's plain fetch-then-ingest
// walk, Lane C's page loop can trigger one FetchByUID PER changed row, so an
// unbounded page (many genuine stragglers clustered together) could burst
// dozens of hydration requests in a single call and trip PlanIt's 429
// threshold outright. Once the cap is reached the walk stops cleanly (the
// same checkpoint-and-return path as a rate limit or hydration error, NOT an
// error itself) so the pass resumes past the already-hydrated rows next
// time, bounding the burst without losing progress.
const maxHydrationsPerPass = 25

// defaultMaxInverseMaskWindowDays is the hard cap on Lane C's rolling
// different=N window (#1127). Live PlanIt probes on 2026-09-06 (end_date
// 2026-06-08, light select, pg_sz=300): different=3 returned ~25k rows in
// ~3.3s — comfortably inside PlanIt's own 45s limit and the 240s handler
// budget — while different=7 hit ~43s once (PlanIt latency is server-load
// dependent, so a larger window is one bad moment from a 45s HTTP 400).
// N is clamped into [2, 3]: N=2 is one full day plus overlap (different=1 is
// only a partial day, ~3.2k rows); N=3 is the ceiling. Days older than the
// cap are a bounded coverage gap recovered by an explicit poll_state reseed
// (Part 3, tc-nkvil) — unbounded catch-up is exactly what livelocked.
//
// A const, not an env var: this is a fixed safety rule, not an operator dial,
// mirroring planit.nationalPageSize's rationale (ADR 0041).
const defaultMaxInverseMaskWindowDays = 3

// InverseMaskOptions tune Lane C's mask cutoff and window cap (ADR 0044 §5,
// as amended by #1127).
type InverseMaskOptions struct {
	// MaskWindow is Lane A's start_date mask width. Lane C's end_date bound
	// is the same cutoff, inverted (today - MaskWindow), so the two lanes
	// partition the national change axis with no gap or overlap. A config
	// dial (POLLING_LANE_A_MASK_DAYS), not a correctness boundary owned by
	// this lane.
	MaskWindow time.Duration
	// MaxWindowDays hard-caps the rolling different=N window. <= 0 means use
	// defaultMaxInverseMaskWindowDays; the handler never surfaces this as a
	// config dial (there is no env var), it exists so tests can pin it.
	MaxWindowDays int
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

// RunOnePage executes exactly one page of Lane C's ascending, bounded
// rolling-window scan (ADR 0044 §5, as amended by #1127): compute this
// cycle's window width N, resume an in-progress same-day scan (with a resume
// overlap, GH#986) at its persisted NextIndex or start a fresh one at index
// 0; fetch one page; diff/hydrate genuinely changed rows up to
// maxHydrationsPerPass; checkpoint.
//
// State reuses the existing PollCursor shape with no schema migration, but
// the semantics changed with #1127:
//   - HighWaterMark holds last_clean_scan_at — when the lane last finished a
//     whole scan with no 429 and no hydration-cap bail. It is what N is
//     recomputed from every cycle (a longer gap → a wider window, hard-capped
//     at maxWindowDays). A zero or stale value yields the cap.
//   - Cursor.DifferentStart holds the in-flight scan's anchor date, valid
//     only while it still equals today: on a calendar-day rollover the
//     rolling window has shifted a full day and PlanIt's total has changed
//     materially, so the persisted index= offset points at different rows —
//     the cursor is discarded and the scan restarts at index 0.
//   - Cursor.NextIndex is the within-scan record offset (unchanged).
//
// GH#986: every early-exit path checkpoints before returning — a page-fetch
// 429/error re-saves the state exactly as loaded with last_poll_time bumped
// to now, and a mid-scan bail (a hydration 429, a straggler error, or the
// hydration cap) saves a cursor at the offset actually reached. Previously
// both paths returned with no save at all, so a persistently-failing record
// froze last_poll_time forever: the planner's LRU then read Lane C as
// perpetually least-recently-polled, picked it every cycle, and it re-walked
// (and re-failed on) the same page — the observed prod livelock that also
// starved Lane A/B of every daytime cycle.
func (h *InverseMaskLaneHandler) RunOnePage(ctx context.Context) laneOutcome {
	ctx, span := otel.Tracer(tracerName).Start(ctx, "PlanIt Lane C inverse-mask poll")
	defer span.End()

	now := h.now().UTC()
	var out laneOutcome

	lastCleanScanAt, _, cursor, err := h.watermark.get(ctx)
	if err != nil {
		out.err = fmt.Errorf("lane C: read scan state: %w", err)
		span.SetAttributes(attribute.String("poll.lane", string(LaneC)))
		return out
	}
	out.watermarkBefore = lastCleanScanAt

	maskCutoff := truncateToDate(now.Add(-h.opts.MaskWindow))

	// N = clamp(days since the last clean scan + 1, 2, maxWindowDays). A zero
	// or long-stale last_clean_scan_at (never run, or a long outage, or the
	// frozen 2026-07 epoch-shaped row from before #1127) yields a large
	// daysSince and therefore the cap — bounded either way, no historical
	// replay, because different=N is inherently N-day bounded.
	maxWindowDays := h.opts.MaxWindowDays
	if maxWindowDays <= 0 {
		maxWindowDays = defaultMaxInverseMaskWindowDays
	}
	windowDays := clampInt(daysSince(now, lastCleanScanAt)+1, 2, maxWindowDays)

	// The cursor is an in-flight scan's checkpoint only while its anchor date
	// is still today; once the day rolls the rolling window has shifted and
	// the offset is meaningless, so drop it and restart at index 0 (mirrors
	// NationalLaneHandler's sameDate(cursor.DifferentStart, watermarkBefore)
	// staleness guard). A re-scan is idempotent (GetByUID dedup).
	var activeCursor *PollCursor
	if cursor != nil && sameDate(cursor.DifferentStart, now) {
		activeCursor = cursor
	}

	// GH#986: resume WITH a safety overlap, mirroring Lane A/B's
	// resumeOverlapRecords. Safe because Lane C dedups every row via
	// GetByUID/inverseMaskDiffers plus the Ingester, so re-processing up to
	// resumeOverlapRecords rows on a resume is idempotent and cheap — and,
	// now that a mid-scan bail checkpoints at the exact failing offset, the
	// overlap is what makes that checkpoint skip-safe against any off-by-one
	// in PlanIt's own ordering.
	startIndex := 0
	if activeCursor != nil {
		startIndex = max(0, activeCursor.NextIndex-resumeOverlapRecords)
	}

	res, ferr := h.fetcher.FetchInverseMaskPage(ctx, planit.NationalInverseMaskQuery{
		WindowDays: windowDays,
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
			out.timedOut = isTimeoutError(ferr)
			out.planitOrigin = true
		}
		// GH#986: re-persist last_clean_scan_at and the cursor exactly as
		// loaded (nothing was fetched, so there is no progress to
		// checkpoint) but with last_poll_time advanced to now, so a
		// page-fetch 429/error still rotates this lane off the LRU front
		// instead of freezing it there.
		if serr := h.watermark.save(ctx, now, lastCleanScanAt, cursor); serr != nil {
			// A save failure is a state-store problem, never PlanIt's fault —
			// join it onto any PlanIt fetch error above and clear
			// planitOrigin, so a genuine persistence failure never gets
			// hidden behind a self-healing PlanIt classification (CodeRabbit
			// follow-up on tc-uitxr).
			out.err = errors.Join(out.err, serr)
			out.planitOrigin = false
		}
		out.watermarkAfter = lastCleanScanAt
		h.recordOutcome(ctx, out)
		h.setSpanAttributes(span, out, windowDays, lastCleanScanAt)
		return out
	}

	out.pages = 1
	out.planitTotal = res.Total
	out.recordsSeen = len(res.Applications)

	// #1127 dropped both last_different boundary checks that the pinned-epoch
	// model needed (the lower-skip at/before epoch_lower and the future-epoch
	// stop past epoch_upper): with no pinned epoch there is no epoch_lower or
	// epoch_upper, and every row different=N returns is within the last N
	// days by construction. Process every row, still subject to
	// inverseMaskDiffers + the hydration cap; re-processing a row seen in a
	// previous overlapping window is idempotent (GetByUID dedup).
	stoppedEarly := false
	hydrationsThisPass := 0
	hydrationCapHit := false
	i := 0
	for ; i < len(res.Applications); i++ {
		light := res.Applications[i]
		if perr := h.processStraggler(ctx, light, &out, &hydrationsThisPass, &hydrationCapHit); perr != nil {
			// timedOut (when applicable) is set inside hydrate itself, not
			// re-derived here from the aggregate perr: processStraggler wraps
			// errors from TWO sources (a Postgres GetByUID read, and a PlanIt
			// hydrate() fetch), and isTimeoutError must only ever be
			// consulted against the PlanIt one (tc-c5tmz, CodeRabbit
			// follow-up on tc-pmh5y).
			out.err = perr
			stoppedEarly = true
			break
		}
		if out.rateLimited || hydrationCapHit {
			// A hydration 429 trips the SAME "stop everything" rule as a
			// page-fetch 429 (ADR 0044: one break on the first 429 from ANY
			// lane) — never follow a rejected request with more requests.
			// The hydration cap (GH#986) is a distinct, non-error reason to
			// stop the same way: it bounds the FetchByUID burst a page of
			// clustered stragglers can trigger.
			stoppedEarly = true
			break
		}
	}

	// The in-flight scan's anchor date: today. A checkpoint saved now is
	// valid only until the next calendar-day rollover (see the staleness
	// guard above).
	scanAnchor := truncateToDate(now)

	if stoppedEarly {
		// GH#986: checkpoint the offset actually reached — startIndex + i,
		// where i counts every record iterated this page — so both the
		// cursor and last_poll_time advance even on a mid-scan bail, and the
		// next pass resumes past what this one already handled instead of
		// re-walking it forever. last_clean_scan_at is left UNCHANGED: this
		// scan did not finish, so N must not reset.
		nextIndex := startIndex + i
		// tc-6u4da: a cluster of permanently-unhydratable rows (FetchByUID
		// genuinely has no matching record for them, forever) never dedupes
		// via GetByUID/inverseMaskDiffers, so every one of them burns a
		// hydration-cap slot on every pass. Because resumeOverlapRecords
		// (100) is much bigger than maxHydrationsPerPass (25), a resume that
		// lands near such a cluster can hit the cap after attempting only
		// maxHydrationsPerPass hydrations — i banked well short of the
		// overlap this resume already subtracted — so the raw startIndex+i
		// checkpoint can land BELOW the cursor this call loaded. Left alone
		// that retreats the persisted cursor, and since the same cluster is
		// still there next pass it retreats again, spiralling backward
		// instead of stalling in place. Clamp to the loaded cursor's own
		// NextIndex (only meaningful when a same-day cursor was actually
		// loaded — a fresh scan has no prior checkpoint to protect) so a
		// stuck cluster can at worst flatline the checkpoint, never walk it
		// backward.
		if activeCursor != nil && nextIndex < activeCursor.NextIndex {
			nextIndex = activeCursor.NextIndex
		}
		newCursor := &PollCursor{DifferentStart: scanAnchor, NextIndex: nextIndex, KnownTotal: res.Total}
		if serr := h.watermark.save(ctx, now, lastCleanScanAt, newCursor); serr != nil {
			// See the page-fetch save above: a save failure is never
			// PlanIt's fault, even when it lands on top of a PlanIt-origin
			// hydration error (CodeRabbit follow-up on tc-uitxr).
			out.err = errors.Join(out.err, serr)
			out.planitOrigin = false
		}
		out.watermarkAfter = lastCleanScanAt
		h.recordOutcome(ctx, out)
		h.setSpanAttributes(span, out, windowDays, lastCleanScanAt)
		return out
	}

	scanComplete := !res.HasMorePages
	spanLastCleanScanAt := lastCleanScanAt

	if scanComplete {
		// Clean scan: reached the last page with no 429 and no hydration-cap
		// bail. Stamp last_clean_scan_at = now (this is what resets N to 2
		// next cycle) and clear the cursor.
		spanLastCleanScanAt = now
		out.watermarkAfter = now
		if serr := h.watermark.save(ctx, now, now, nil); serr != nil && out.err == nil {
			out.err = serr
		}
	} else {
		// More pages remain this cycle: checkpoint the within-scan offset,
		// leave last_clean_scan_at unchanged (the scan is not finished).
		nextIndex := startIndex + len(res.Applications)
		// tc-6u4da: same monotonic clamp as the stoppedEarly branch — a
		// fully-consumed page's nextIndex is normally well past the prior
		// checkpoint already, but monotonic advance should hold everywhere
		// this lane persists NextIndex.
		if activeCursor != nil && nextIndex < activeCursor.NextIndex {
			nextIndex = activeCursor.NextIndex
		}
		newCursor := &PollCursor{DifferentStart: scanAnchor, NextIndex: nextIndex, KnownTotal: res.Total}
		out.watermarkAfter = lastCleanScanAt
		if serr := h.watermark.save(ctx, now, lastCleanScanAt, newCursor); serr != nil && out.err == nil {
			out.err = serr
		}
	}

	h.recordOutcome(ctx, out)
	h.setSpanAttributes(span, out, windowDays, spanLastCleanScanAt)
	return out
}

// daysSince returns the whole-day gap between now and earlier, measured on
// their truncated-to-date (UTC midnight) values and floored at zero. A zero
// earlier yields a very large number (its date is year 1) — Time.Sub
// saturates rather than overflowing — which the caller clamps to the window
// cap.
func daysSince(now, earlier time.Time) int {
	d := truncateToDate(now).Sub(truncateToDate(earlier))
	if d <= 0 {
		return 0
	}
	return int(d / (24 * time.Hour))
}

// clampInt confines x to [lo, hi]. lo must not exceed hi.
func clampInt(x, lo, hi int) int {
	return max(lo, min(x, hi))
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
//
// hydrationsThisPass/hydrationCapHit (GH#986) bound the FetchByUID fan-out a
// single RunOnePage call can trigger: once hydrationsThisPass reaches
// maxHydrationsPerPass, a row that would otherwise hydrate is left alone
// (hydrationCapHit set instead) so the caller stops the walk and checkpoints
// at this exact record rather than burst-hydrating an unbounded page of
// clustered stragglers. A row that dedupes via GetByUID/inverseMaskDiffers
// never counts against the cap — only a genuine FetchByUID attempt does.
func (h *InverseMaskLaneHandler) processStraggler(ctx context.Context, light applications.PlanningApplication, out *laneOutcome, hydrationsThisPass *int, hydrationCapHit *bool) error {
	authorityCode := strconv.Itoa(light.AreaID)
	existing, found, gerr := h.apps.GetByUID(ctx, light.UID, authorityCode)
	if gerr != nil {
		return fmt.Errorf("lane C: read existing application %q (authority %s): %w", light.UID, authorityCode, gerr)
	}
	if found && !inverseMaskDiffers(existing, light) {
		return nil
	}
	if *hydrationsThisPass >= maxHydrationsPerPass {
		*hydrationCapHit = true
		return nil
	}
	*hydrationsThisPass++
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
		// timedOut/planitOrigin are set here, at the actual PlanIt fetch site,
		// rather than re-derived from processStraggler's aggregate error at
		// the RunOnePage call site -- so a Postgres GetByUID error (the
		// sibling error source processStraggler wraps) never gets
		// misclassified as PlanIt-origin (tc-c5tmz, CodeRabbit follow-up on
		// tc-pmh5y; tc-uitxr).
		out.timedOut = isTimeoutError(err)
		out.planitOrigin = true
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
// OldestHighWaterMarkAge: its "watermark" is last_clean_scan_at, which by
// design tracks recent completions rather than backlog depth (a wider
// different=N window is the bounded response to a longer gap, not an
// ever-growing age), so an age-to-now gauge would mislead — mirroring
// BackfillHandler's identical omission for Lane D and its stated rationale.
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
// mirroring the other lanes' setSpanAttributes. windowDays is the rolling
// different=N width used this cycle; lastCleanScanAt is the value in effect
// after this call (advanced to now on a clean scan, unchanged otherwise), so
// spans can be grouped to check the records_seen == planit.total invariant
// and to see how far Lane C has drifted from a clean completion.
func (h *InverseMaskLaneHandler) setSpanAttributes(span trace.Span, out laneOutcome, windowDays int, lastCleanScanAt time.Time) {
	attrs := []attribute.KeyValue{
		attribute.String("poll.lane", string(LaneC)),
		attribute.Int("poll.records_seen", out.recordsSeen),
		attribute.Int("poll.records_ingested", out.recordsIngested),
		attribute.Int("poll.pages", out.pages),
		attribute.Int("poll.window_days", windowDays),
		attribute.String("poll.last_clean_scan_at", formatWatermark(lastCleanScanAt)),
		attribute.Bool("poll.rate_limited", out.rateLimited),
	}
	if out.planitTotal != nil {
		attrs = append(attrs, attribute.Int("planit.total", *out.planitTotal))
	}
	if out.err != nil {
		attrs = append(attrs, attribute.String("poll.error", out.err.Error()))
	}
	span.SetAttributes(attrs...)
}
