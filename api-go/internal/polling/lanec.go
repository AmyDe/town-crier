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
//
// tc-hku56 (GH#1140) removed the second layer of that same problem: this
// file's page loop used to trigger one separate id_match hydration request
// per differing row, and PlanIt's rate limiter never tolerated the resulting
// burst — a complete different=3 scan cost ~138 page fetches plus ~12,400
// hydration requests, so a page routinely 429'd after its 11th hydration and
// Lane C never got past ~5% of its own window before the checkpoint froze.
// The page fetch now requests planit.ingestSelectFields directly (the same
// full projection Lane D already runs in prod), and processStraggler ingests
// the page row itself: the only PlanIt request RunOnePage makes is the page
// fetch. See InverseMaskOptions.MaxPages (the per-cycle page cap this makes
// necessary) and the ADR 0044 second amendment.
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
// needs: one ascending rolling-window page. *planit.Client satisfies it. Its
// FetchByUID method is deliberately NOT part of this interface (tc-hku56 /
// GH#1140 removed Lane C's per-record id_match hydration fan-out entirely —
// Lane E still calls FetchByUID directly, via its own recentSweepFetcher).
type inverseMaskFetcher interface {
	FetchInverseMaskPage(ctx context.Context, q planit.NationalInverseMaskQuery) (planit.FetchPageResult, error)
}

// laneCResumeOverlapRecords is the record overlap Lane C subtracts from a
// resumed scan's persisted NextIndex (it resumes at
// max(0, NextIndex-laneCResumeOverlapRecords)) to tolerate PlanIt
// record-shift between cycles — Lane C's own, deliberately smaller
// counterpart to the shared resumeOverlapRecords (100, handler.go) that
// Lanes A/B and the legacy per-authority drain use.
//
// tc-hku56 retired this constant's original rationale. Before the page
// projection widened to ingestSelectFields, a resume landing on a cluster of
// permanently-unhydratable rows (cross-authority uid collisions PlanIt
// resolved to the wrong authority, e.g. area 198 (Bassetlaw) vs area 301
// (Croydon), bead tc-777e7 Bug 2) burned one FetchByUID-hydration-cap slot
// per phantom row on every pass, and such a row never deduped via
// GetByUID/inverseMaskDiffers — so the overlap had to stay strictly below the
// old maxHydrationsPerPass cap, or the pass would re-spend its whole
// hydration budget re-failing rows it already walked and the tc-6u4da
// anti-retreat clamp would pin the cursor there forever (the observed
// 2026-07 → 2026-09 prod livelock). With the id_match hydration fan-out gone
// there is no hydration cap left for the overlap to stay under, and no
// per-record PlanIt request that can fail: GetByUID/Ingest errors are
// Postgres-origin and dedupe identically on a resume. The value stays at 10
// simply as a sensible small tolerance for PlanIt's own record-shift across
// the intra-day gap between one cycle's page fetch and the next's; changing
// the number is unnecessary churn.
//
// A const, not an env var: a fixed safety rule, mirroring planit.nationalPageSize
// (ADR 0041).
const laneCResumeOverlapRecords = 10

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

// InverseMaskOptions tune Lane C's mask cutoff, window cap, per-cycle page
// budget and notification recency gate (ADR 0044 §5, as amended by #1127 and
// tc-hku56 / GH#1140).
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
	// MaxPages bounds how many pages of Lane C NationalPollHandler.Handle
	// will run within a single Handle call (nil = unbounded), mirroring
	// NationalLaneOptions.MaxPages: NationalPollHandler.loadPlannerState
	// excludes a lane that has already run MaxPages pages this cycle from
	// planner candidacy for the rest of THIS cycle, without touching its real
	// persisted state, so the walk resumes exactly where it left off next
	// cycle via the persisted cursor. Necessary now that a page costs one
	// request instead of triggering an id_match hydration burst that used to
	// 429 the page loop shut on its own: uncapped, the handler budget would
	// let Lane C fire far more requests in one cycle than the burst that
	// caused the tc-vgbl7 rollback. Config: POLLING_LANE_C_MAX_PAGES_PER_CYCLE,
	// default 15 (sized for a worst-case 138-page different=3 scan to finish
	// inside the ~12-cycle daytime window).
	MaxPages *int
	// NotifyRecencyWindow feeds the recency gate WithFanOut composes around
	// both fan-out collaborators (recencyGatedDispatcher / recencyGatedEnqueuer,
	// ADR 0047's gate — see recentsweep_gate.go), mirroring Lane E's
	// RecentSweepOptions.NotifyRecencyWindow: an event older than this (by
	// start_date for a new application, decided_date for a decision) produces
	// no notification record at all. Config: POLLING_LANE_C_NOTIFY_RECENCY_DAYS,
	// default 30. Lane C's band is start_date <= today-90d by construction, so
	// this suppresses ALL NewApplication fan-out from Lane C — intended, not a
	// regression: an application filed 90+ days ago is not new to anybody. A
	// genuine recent decision on an old application still dispatches, which is
	// the backstop role Lane C exists for.
	NotifyRecencyWindow time.Duration
}

// InverseMaskLaneHandler runs ADR 0044's Lane C: one page per call of a
// national, ascending, bounded rolling different=N window inverse-mask query
// (§5, as amended by #1127 and tc-hku56 / GH#1140) — the complement of
// Lane A/B's masked-delta band, reconciling old applications' status drift
// the delta axis structurally cannot see. Diffs each row against Postgres on
// app_state and decided_date only (last_different is DROPPED from the diff —
// PlanIt bumps it on every re-index, so keeping it would flag every churned
// old record as a straggler, the old per-authority lane's measured
// hydration-amplification bug) and ingests only genuine changes directly
// from the already-full page row (tc-hku56: no separate hydration request).
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

// WithFanOut wraps the incoming notification fan-out collaborators in Lane
// C's own recency decorators (recencyGatedDispatcher / recencyGatedEnqueuer,
// ADR 0047's gate — see recentsweep_gate.go) ITSELF, then wires them onto the
// handler's Ingester, exactly as RecentSweepHandler.WithFanOut does
// (tc-hku56 / GH#1140): the wiring site cannot hand Lane C an ungated
// notifier, because the gate is applied inside this setter rather than at the
// call site. Includes the nil-ingester guard, so calling this on a zero-value
// InverseMaskLaneHandler never panics. Returns the handler for chaining.
func (h *InverseMaskLaneHandler) WithFanOut(decision DecisionDispatcher, enqueuer NotificationEnqueuer) *InverseMaskLaneHandler {
	if h.ingester == nil {
		h.ingester = &Ingester{}
	}
	h.ingester.decision = recencyGatedDispatcher{inner: decision, window: h.opts.NotifyRecencyWindow, now: h.now}
	h.ingester.enqueuer = recencyGatedEnqueuer{inner: enqueuer, window: h.opts.NotifyRecencyWindow, now: h.now}
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
// rolling-window scan (ADR 0044 §5, as amended by #1127 and tc-hku56 /
// GH#1140): compute this cycle's window width N, resume an in-progress
// same-day scan (with a resume overlap, GH#986) at its persisted NextIndex or
// start a fresh one at index 0; fetch one page (the full ingestSelectFields
// projection — tc-hku56, no separate hydration request); diff every row
// against Postgres and ingest genuinely changed rows directly from the page;
// checkpoint. The only PlanIt request this call ever makes is the one page
// fetch, so a mid-page 429 is now structurally impossible — the per-cycle
// page budget lives in InverseMaskOptions.MaxPages instead (enforced by the
// caller, NationalPollHandler.loadPlannerState).
//
// State reuses the existing PollCursor shape with no schema migration, but
// the semantics changed with #1127 and again with #1171:
//   - HighWaterMark holds the coverage watermark: last_clean_scan_at, folded
//     forward with a stale cursor's WalkHead when a fresh scan starts, and
//     advanced to now only once a scan completes. It is what N is recomputed
//     from every cycle, and it moves ONLY at these two scan boundaries —
//     never mid-scan — because changing it mid-scan would recompute N under
//     an already-persisted index= offset and skip rows.
//   - Cursor.DifferentStart holds the in-flight scan's anchor date, valid
//     only while it still equals today: on a calendar-day rollover the
//     rolling window has shifted a full day and PlanIt's total has changed
//     materially, so the persisted index= offset points at different rows —
//     the cursor is discarded and the scan restarts at index 0.
//   - Cursor.NextIndex is the within-scan record offset (unchanged).
//   - Cursor.WalkHead is this scan's in-flight coverage head: the maximum
//     LastDifferent over rows the scan has checked so far, folded into
//     HighWaterMark when a stale cursor is discarded at a fresh start.
//
// GH#986: every early-exit path checkpoints before returning — a page-fetch
// 429/error re-saves the state exactly as loaded with last_poll_time bumped
// to now, and a mid-scan bail (a processStraggler error: a Postgres GetByUID
// read or an Ingest failure) saves a cursor at the offset actually reached.
// Previously both paths returned with no save at all, so a
// persistently-failing record froze last_poll_time forever: the planner's LRU
// then read Lane C as perpetually least-recently-polled, picked it every
// cycle, and it re-walked (and re-failed on) the same page — the observed
// prod livelock that also starved Lane A/B of every daytime cycle.
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

	// The cursor is an in-flight scan's checkpoint only while its anchor date
	// is still today; once the day rolls the rolling window has shifted and
	// the offset is meaningless, so drop it and restart at index 0 (mirrors
	// NationalLaneHandler's sameDate(cursor.DifferentStart, watermarkBefore)
	// staleness guard). A re-scan is idempotent (GetByUID dedup).
	var activeCursor *PollCursor
	if cursor != nil && sameDate(cursor.DifferentStart, now) {
		activeCursor = cursor
	}

	// coverageMark (GH#1171) folds a discarded stale cursor's in-flight
	// coverage head into the watermark N is computed from, so a scan that
	// never finished before the day rolled still counts the progress it
	// made. It is the value this call treats as last_clean_scan_at
	// everywhere below; it only ever moves here (a fresh-start fold) or on a
	// completed scan (advanced to now), never mid-scan.
	coverageMark := lastCleanScanAt
	if cursor != nil && activeCursor == nil && cursor.WalkHead.After(coverageMark) {
		coverageMark = cursor.WalkHead
	}

	// N = clamp(days since the coverage mark + 1, 2, maxWindowDays). A zero
	// or long-stale mark (never run, or a long outage, or the frozen 2026-07
	// epoch-shaped row from before #1127) yields a large daysSince and
	// therefore the cap — bounded either way, no historical replay, because
	// different=N is inherently N-day bounded.
	maxWindowDays := h.opts.MaxWindowDays
	if maxWindowDays <= 0 {
		maxWindowDays = defaultMaxInverseMaskWindowDays
	}
	windowDays := clampInt(daysSince(now, coverageMark)+1, 2, maxWindowDays)

	// GH#986: resume WITH a safety overlap. Lane C uses its OWN
	// laneCResumeOverlapRecords (10), deliberately smaller than Lane A/B's
	// shared resumeOverlapRecords (100) — see its doc comment for why the
	// value stays small even though the hydration-cap rationale that used to
	// bound it from above no longer applies. Safe because Lane C dedups every
	// row via GetByUID/inverseMaskDiffers plus the Ingester, so re-processing
	// up to laneCResumeOverlapRecords rows on a resume is idempotent and cheap
	// — and, now that a mid-scan bail checkpoints at the exact failing offset,
	// the overlap is what makes that checkpoint skip-safe against any
	// off-by-one in PlanIt's own ordering.
	startIndex := 0
	var head time.Time
	if activeCursor != nil {
		startIndex = max(0, activeCursor.NextIndex-laneCResumeOverlapRecords)
		head = activeCursor.WalkHead
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
		// GH#986: re-persist the coverage mark and the cursor exactly as
		// loaded (nothing was fetched, so there is no progress to
		// checkpoint) but with last_poll_time advanced to now, so a
		// page-fetch 429/error still rotates this lane off the LRU front
		// instead of freezing it there.
		if serr := h.watermark.save(ctx, now, coverageMark, cursor); serr != nil {
			// A save failure is a state-store problem, never PlanIt's fault —
			// join it onto any PlanIt fetch error above and clear
			// planitOrigin, so a genuine persistence failure never gets
			// hidden behind a self-healing PlanIt classification (CodeRabbit
			// follow-up on tc-uitxr).
			out.err = errors.Join(out.err, serr)
			out.planitOrigin = false
		}
		out.watermarkAfter = coverageMark
		h.recordOutcome(ctx, out)
		h.setSpanAttributes(span, out, windowDays, coverageMark, head)
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
	// inverseMaskDiffers; re-processing a row seen in a previous overlapping
	// window is idempotent (GetByUID dedup).
	//
	// tc-hku56: with the id_match hydration fan-out gone, processStraggler
	// makes no PlanIt request at all — only a Postgres GetByUID read and, on
	// a genuine diff, an Ingest call — so a mid-page 429 can no longer
	// happen. stoppedEarly now fires only on a processStraggler error
	// (never PlanIt-origin: isTimeoutError/out.planitOrigin are never
	// consulted on this path, tc-c5tmz), keeping the existing
	// checkpoint-and-clamp behaviour below for that one remaining case.
	//
	// GH#1171: head tracks the ascending scan's own coverage progress — the
	// max LastDifferent over rows processStraggler has actually checked. A
	// row only counts once it succeeds, so a hard stop below leaves head at
	// the last row actually checked, never the failing one; a zero
	// LastDifferent (never a genuine PlanIt value) never advances it.
	stoppedEarly := false
	i := 0
	for ; i < len(res.Applications); i++ {
		light := res.Applications[i]
		if perr := h.processStraggler(ctx, light, &out); perr != nil {
			out.err = perr
			stoppedEarly = true
			break
		}
		if light.LastDifferent.After(head) {
			head = light.LastDifferent
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
		// re-walking it forever. The coverage mark is left UNCHANGED: this
		// scan did not finish, so N must not reset.
		nextIndex := startIndex + i
		// tc-6u4da: the persisted cursor must never retreat. A processStraggler
		// error (a Postgres GetByUID read or an Ingest failure) that breaks the
		// loop within the first laneCResumeOverlapRecords rows of a resume
		// leaves i < laneCResumeOverlapRecords, so startIndex+i is below the
		// loaded cursor. Clamp to the loaded cursor's own NextIndex (only
		// meaningful when a same-day cursor was actually loaded — a fresh scan
		// has no prior checkpoint to protect) so such an early bail holds the
		// cursor in place, never walks it backward.
		if activeCursor != nil && nextIndex < activeCursor.NextIndex {
			nextIndex = activeCursor.NextIndex
		}
		newCursor := &PollCursor{DifferentStart: scanAnchor, NextIndex: nextIndex, KnownTotal: res.Total, WalkHead: head}
		if serr := h.watermark.save(ctx, now, coverageMark, newCursor); serr != nil {
			// See the page-fetch save above: a save failure is never
			// PlanIt's fault, even when it lands on top of a PlanIt-origin
			// hydration error (CodeRabbit follow-up on tc-uitxr).
			out.err = errors.Join(out.err, serr)
			out.planitOrigin = false
		}
		out.watermarkAfter = coverageMark
		h.recordOutcome(ctx, out)
		h.setSpanAttributes(span, out, windowDays, coverageMark, head)
		return out
	}

	scanComplete := !res.HasMorePages
	spanCoverageMark := coverageMark

	if scanComplete {
		// Clean scan: reached the last page with no 429 and no
		// processStraggler error. Stamp the coverage mark = now (this is
		// what resets N to 2 next cycle) and clear the cursor.
		spanCoverageMark = now
		out.watermarkAfter = now
		if serr := h.watermark.save(ctx, now, now, nil); serr != nil && out.err == nil {
			out.err = serr
		}
	} else {
		// More pages remain this cycle: checkpoint the within-scan offset
		// and coverage head, leave the coverage mark unchanged (the scan is
		// not finished).
		nextIndex := startIndex + len(res.Applications)
		// tc-6u4da: same monotonic clamp as the stoppedEarly branch — a
		// fully-consumed page's nextIndex is normally well past the prior
		// checkpoint already, but monotonic advance should hold everywhere
		// this lane persists NextIndex.
		if activeCursor != nil && nextIndex < activeCursor.NextIndex {
			nextIndex = activeCursor.NextIndex
		}
		newCursor := &PollCursor{DifferentStart: scanAnchor, NextIndex: nextIndex, KnownTotal: res.Total, WalkHead: head}
		out.watermarkAfter = coverageMark
		if serr := h.watermark.save(ctx, now, coverageMark, newCursor); serr != nil && out.err == nil {
			out.err = serr
		}
	}

	h.recordOutcome(ctx, out)
	h.setSpanAttributes(span, out, windowDays, spanCoverageMark, head)
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

// processStraggler diffs one inverse-mask page row against Postgres on
// app_state and decided_date only (existence counts as a difference too) and,
// when it genuinely differs, ingests the row DIRECTLY (tc-hku56 / GH#1140:
// the page row already carries the full ingestSelectFields projection, so
// there is no second PlanIt request to make). authorityCode is built from the
// row's area_id (ADR 0044's national-query correctness fix): PlanIt's uid is
// only unique within one authority, so a national query cannot diff/ingest by
// uid alone without an authority scope, or two authorities sharing a bare uid
// could cross-contaminate. Any local failure (the existence read, or the
// Ingest itself) is a hard stop — out.err set, mirroring Lane A/B's own
// "never silently skip, freeze and let the next call resume" behaviour — so
// it is surfaced as an error rather than logged-and-skipped, unlike the
// deleted per-authority ReconciliationHandler. Neither error source is
// PlanIt-origin: isTimeoutError/out.planitOrigin are never consulted here
// (tc-c5tmz) — only the page fetch above can set them.
func (h *InverseMaskLaneHandler) processStraggler(ctx context.Context, light applications.PlanningApplication, out *laneOutcome) error {
	authorityCode := strconv.Itoa(light.AreaID)
	existing, found, gerr := h.apps.GetByUID(ctx, light.UID, authorityCode)
	if gerr != nil {
		return fmt.Errorf("lane C: read existing application %q (authority %s): %w", light.UID, authorityCode, gerr)
	}
	if found && !inverseMaskDiffers(existing, light) {
		return nil
	}
	if ierr := h.ingester.Ingest(ctx, light); ierr != nil {
		return fmt.Errorf("lane C: ingest %q: %w", light.UID, ierr)
	}
	out.recordsIngested++
	return nil
}

// inverseMaskDiffers reports whether the inverse-mask row's app_state or
// decided_date differs from the persisted application — Lane C's straggler
// test (ADR 0044 §4). last_different is DELIBERATELY excluded: PlanIt bumps
// it on every re-index, so comparing it would flag every churned old record
// as a straggler and ingest it for nothing — the measured
// ingest-amplification bug the old per-authority lane hit (a hydration
// fan-out at the time; the same anti-churn contract applies unchanged to the
// direct-ingest model). A last_different-only churned row must NOT ingest.
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
// different=N width used this cycle; lastCleanScanAt is the coverage mark in
// effect after this call (advanced to now on a clean scan, folded forward
// from a stale cursor's head at a fresh start, unchanged otherwise);
// coverageHead is the in-flight scan's own progress (GH#1171), empty when no
// scan is active. Spans can be grouped to check the records_seen ==
// planit.total invariant and to see how far Lane C has drifted from a clean
// completion.
func (h *InverseMaskLaneHandler) setSpanAttributes(span trace.Span, out laneOutcome, windowDays int, lastCleanScanAt, coverageHead time.Time) {
	attrs := []attribute.KeyValue{
		attribute.String("poll.lane", string(LaneC)),
		attribute.Int("poll.records_seen", out.recordsSeen),
		attribute.Int("poll.records_ingested", out.recordsIngested),
		attribute.Int("poll.pages", out.pages),
		attribute.Int("poll.window_days", windowDays),
		attribute.String("poll.last_clean_scan_at", formatWatermark(lastCleanScanAt)),
		attribute.String("poll.coverage_head", formatWatermark(coverageHead)),
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
