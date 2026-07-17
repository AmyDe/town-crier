package polling

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"

	"github.com/AmyDe/town-crier/api-go/internal/applications"
	"github.com/AmyDe/town-crier/api-go/internal/planit"
)

// reconciliationFetcher is the consumer-side slice of the PlanIt client Lane C
// needs: one light per-authority projection page, and a full-record hydration
// fetch by uid. *planit.Client satisfies both.
type reconciliationFetcher interface {
	FetchReconciliationPage(ctx context.Context, authorityID, startIndex int, differentStart time.Time) (planit.FetchPageResult, error)
	FetchByUID(ctx context.Context, uid string) (planit.FetchPageResult, error)
}

// ReconciliationOptions tune Lane C's cadence and blast radius (ADR 0041).
type ReconciliationOptions struct {
	// Interval is the minimum gap between full sweeps of every pollable
	// authority ("daily during soak, then weekly" per the ADR's migration
	// plan — a config dial, not a correctness boundary: Lane C is a
	// completeness backstop, not the cutover's verification mechanism).
	Interval time.Duration
	// MaxStragglersPerAuthority bounds the per-authority hydration fan-out so
	// one badly-drifted authority cannot balloon a single sweep's PlanIt
	// request count.
	MaxStragglersPerAuthority int
	// AuthoritiesPerCycle bounds how many authorities a single Run call
	// sweeps (config: POLLING_LANE_C_AUTHORITIES_PER_CYCLE, default 50 — 50 x
	// 2s throttle = 100s, comfortably inside the ~570s cycle budget; a full
	// ~485-authority pass takes ~10 cycles). A pass that doesn't finish this
	// cycle persists a resumable cursor (see Run) so the next cycle picks up
	// where this one left off instead of restarting at authority 0 — without
	// this bound, and without the resume, every weekly attempt would only
	// ever reach the authorities that fit one cycle's budget, starving the
	// rest permanently (tc-tuge8/GH#971).
	AuthoritiesPerCycle int
	// LookbackDays bounds how far back Lane C's different_start prefilter
	// reaches (config: POLLING_LANE_C_LOOKBACK_DAYS, default 365). PlanIt
	// rejects a reconciliation query with no date bound at all --
	// "Spatial, date or search restrictions required in query", the
	// tc-tuge8/GH#971 root cause confirmed from prod's
	// reconciliation.sample_error_body span attribute. Deliberately generous
	// and NOT a churn mask (see MaskStartDate/MaskDecidedStart on
	// planit.NationalDeltaQuery): Lane C's job is detecting drift in what
	// PlanIt has touched, not computing an exact delta, so a wide, date-only
	// prefilter satisfies PlanIt's requirement while still scanning each
	// authority's genuinely-touched-recently set.
	LookbackDays int
}

// ReconciliationHandler runs ADR 0041's Lane C: per authority, fetch a light
// projection of its live set (uid, app_state, decided_date, last_different),
// diff each row against the persisted application, and hydrate (a
// full-record fetch, then the standard Ingest) only the rows that genuinely
// differ. It is deliberately NOT the cutover's verification mechanism — that
// is the planit.total / records_seen invariant stamped on Lane A/B's own
// spans. Lane C is a completeness backstop for what the delta axis
// structurally cannot see: decisions with no decided_date, rows with no
// app_state, and applications discovered so late their start_date falls
// outside both masks.
type ReconciliationHandler struct {
	fetcher     reconciliationFetcher
	watermark   *laneWatermarkStore
	ingester    *Ingester
	authorities activeAuthorityProvider
	opts        ReconciliationOptions
	now         func() time.Time
	logger      *slog.Logger

	// metrics records towncrier.polling.applications_ingested (tagged "C")
	// for this lane's sweeps, wired via WithMetrics, mirroring
	// PollPlanItHandler.metrics. nil until wired (the no-metrics default).
	metrics metricsRecorder
}

// NewReconciliationHandler wires Lane C. now is injected so tests pin the
// clock.
func NewReconciliationHandler(
	fetcher reconciliationFetcher,
	state pollStateAccess,
	apps applicationStore,
	authorityProvider activeAuthorityProvider,
	opts ReconciliationOptions,
	now func() time.Time,
	logger *slog.Logger,
) *ReconciliationHandler {
	return &ReconciliationHandler{
		fetcher:     fetcher,
		watermark:   newLaneWatermarkStore(state, sentinelLaneC),
		ingester:    NewIngester(apps, nil, nil),
		authorities: authorityProvider,
		opts:        opts,
		now:         now,
		logger:      logger,
	}
}

// WithFanOut wires the notification fan-out collaborators onto Lane C's
// hydration ingests, mirroring PollPlanItHandler.WithFanOut (including its
// nil-ingester guard, so calling this on a zero-value ReconciliationHandler —
// as a wiring test does — never panics). Returns the handler for chaining.
func (h *ReconciliationHandler) WithFanOut(decision DecisionDispatcher, enqueuer NotificationEnqueuer) *ReconciliationHandler {
	if h.ingester == nil {
		h.ingester = &Ingester{}
	}
	h.ingester.decision = decision
	h.ingester.enqueuer = enqueuer
	return h
}

// WithMetrics wires the metrics recorder Lane C records its per-sweep
// ApplicationsIngested count on, mirroring PollPlanItHandler.WithMetrics. A
// post-construction setter, so the many tests that don't supply one are
// unaffected; cmd/worker calls it once after construction. Returns the
// handler for chaining.
func (h *ReconciliationHandler) WithMetrics(rec metricsRecorder) *ReconciliationHandler {
	h.metrics = rec
	return h
}

// recorder returns a non-nil recorder so call sites can record
// unconditionally, mirroring PollPlanItHandler.recorder.
func (h *ReconciliationHandler) recorder() metricsRecorder {
	if h.metrics == nil {
		return noopMetrics{}
	}
	return h.metrics
}

// Due reports whether Lane C should run this cycle: either a sweep pass is
// already mid-flight (a persisted cursor), in which case it continues
// unconditionally regardless of Interval, or Interval has elapsed since the
// last COMPLETED pass (or none has ever completed). A watermark-store read
// error fails safe (not due): skipping a sweep this cycle is far cheaper than
// risking an unbounded extra PlanIt load off the back of a store hiccup.
func (h *ReconciliationHandler) Due(ctx context.Context, now time.Time) bool {
	_, lastRun, cursor, err := h.watermark.get(ctx)
	if err != nil {
		h.logger.WarnContext(ctx, "lane C: read last-run watermark failed; skipping this cycle's due check", "error", err)
		return false
	}
	if cursor != nil {
		return true
	}
	return lastRun.IsZero() || now.Sub(lastRun) >= h.opts.Interval
}

// reconciliationOutcome is one Lane C sweep's result, carried onto the
// telemetry span.
type reconciliationOutcome struct {
	authoritiesSwept int
	recordsSeen      int
	stragglers       int
	hydrated         int
	err              error
	// sampleErrorBody is the FIRST non-empty PlanIt HTTP error body observed
	// this sweep, across every authority's fetch error. It is never
	// overwritten once set: one sample is enough to read a 400's reason off
	// AppDependencies (AppTraces is Basic-tier and invisible to KQL, so this
	// travels on the span, not a log line -- tc-tuge8/GH#971).
	sampleErrorBody string
	// badRequestCount counts fetch errors specifically carrying HTTP 400
	// (reconciliation.error_count on the span). Other statuses (5xx,
	// transport failures) are logged per-authority as today but don't
	// inflate this counter -- it exists to answer "is every authority 400ing
	// the same way", the v0.21.0 storm signature.
	badRequestCount int
	// rateLimited is set once, the first time ANY PlanIt call this Run makes
	// -- a per-authority sweep fetch (FetchReconciliationPage) or a straggler
	// hydration fetch (FetchByUID) -- returns a *planit.RateLimitError, and
	// is never cleared afterwards (tc-mc0hf). It is Lane C's circuit breaker:
	// once tripped, sweepAuthority stops hydrating further stragglers for the
	// rest of that authority and Run stops sweeping further authorities for
	// the rest of this cycle, mirroring NationalLaneHandler's "stop on first
	// 429" behavior for Lane A/B. Without this, a single 429 was previously
	// followed by every remaining straggler hydration and authority sweep
	// still firing anyway -- 141 of 187 hydration requests rejected with 429
	// over 8.5 minutes in one prod cycle, a live violation of PlanIt's
	// never-hammer red line.
	rateLimited bool
}

// Run sweeps up to AuthoritiesPerCycle authorities' light projections, diffs
// each row against Postgres, and hydrates genuinely-differing rows one uid at
// a time (bulk hydration by id_match is unproven — ADR 0041). A per-authority
// fetch error is logged and skipped, not fatal to the sweep: the next
// scheduled sweep retries it.
//
// The full ~485-authority pass rarely fits one cycle's budget, so Run resumes
// a persisted authority-list cursor across cycles rather than restarting at
// index 0 every time (tc-tuge8/GH#971): without a resume, only the
// authorities that fit inside one cycle's budget would EVER be swept, and the
// rest would starve permanently — the same failure class ADR 0041 was
// written to kill. The cursor lives in the same poll_state row Due reads (the
// -3 sentinel), so a mid-pass cycle is unconditionally Due regardless of
// Interval; Interval only gates starting a brand-new pass once the previous
// one has completed.
func (h *ReconciliationHandler) Run(ctx context.Context) reconciliationOutcome {
	ctx, span := otel.Tracer(tracerName).Start(ctx, "PlanIt reconciliation sweep")
	defer span.End()

	now := h.now().UTC()
	var out reconciliationOutcome

	ids, err := h.authorities.ActiveAuthorityIDs(ctx)
	if err != nil {
		out.err = fmt.Errorf("lane C: list authorities: %w", err)
		span.SetAttributes(attribute.String("poll.lane", string(LaneC)))
		// out is not fully computed here (the authority list itself failed to
		// load, so nothing was swept) — no metrics recorded, mirroring the
		// absence of the full span attribute set on this early bail-out.
		return out
	}

	_, lastRun, cursor, werr := h.watermark.get(ctx)
	if werr != nil {
		out.err = fmt.Errorf("lane C: read watermark: %w", werr)
		span.SetAttributes(attribute.String("poll.lane", string(LaneC)))
		return out
	}

	start := 0
	if cursor != nil && cursor.NextIndex < len(ids) {
		start = cursor.NextIndex
	}
	// A stale cursor at or past the current authority list's end (e.g. the
	// list shrank between cycles) falls through to start=0 above rather than
	// sweeping zero authorities forever.
	end := min(start+h.opts.AuthoritiesPerCycle, len(ids))

	// The different_start cutoff is computed ONCE per sweep from this Run
	// call's own injected clock (now) and threaded down to every authority's
	// fetch, mirroring how NationalLaneHandler.Run computes MaskCutoff once
	// rather than per-page/per-authority (tc-tuge8/GH#971).
	cutoff := now.AddDate(0, 0, -h.opts.LookbackDays)

	attempted := start
	for _, authorityID := range ids[start:end] {
		if ctx.Err() != nil {
			break
		}
		h.sweepAuthority(ctx, authorityID, cutoff, &out)
		attempted++
		// The authority just swept above DID make its request (and is
		// correctly counted in attempted immediately above) even when that
		// request came back rate-limited -- only the NEXT authority's sweep
		// is skipped. Mirrors NationalLaneHandler's "stop on first 429" and
		// feeds the exact same actually-attempted persistence path as the
		// ctx.Err() break above (tc-mc0hf).
		if out.rateLimited {
			break
		}
	}

	// The persisted next-index must reflect how many authorities were
	// ACTUALLY attempted before any early ctx-cancel break above, not the
	// planned slice end (end) — otherwise the never-attempted tail between
	// them is skipped forever, reproducing the exact starvation this cursor
	// exists to fix.
	newLastRun := lastRun
	var newCursor *PollCursor
	if attempted >= len(ids) {
		// The pass reached the end of the authority list: complete. Reset the
		// cursor and stamp last-run now — the only branch that advances
		// LastPollTime, so Due's Interval math is measured from the last
		// COMPLETED pass, not from a mid-pass cycle.
		newLastRun = now
	} else {
		// Still mid-pass: persist the resume position. DifferentStart and
		// KnownTotal are Lane A/B pagination concepts (a date-bound resume
		// position within one authority's PlanIt page walk) that don't apply
		// to Lane C's plain index into the static authority list, so both
		// stay zero/nil.
		newCursor = &PollCursor{NextIndex: attempted}
	}

	// Persist uncancellably: a budget-cutoff cancellation of the request ctx
	// must never lose this cycle's write (root cause 2, tc-tuge8/GH#971) —
	// without this, Due sees no persisted state forever and Lane C re-fires
	// every cycle, storming PlanIt. Lane C's watermark row carries no delta
	// boundary of its own (HighWaterMark always stays the zero time); only
	// LastPollTime and Cursor move.
	saveCtx := context.WithoutCancel(ctx)
	if serr := h.watermark.save(saveCtx, newLastRun, time.Time{}, newCursor); serr != nil && out.err == nil {
		out.err = serr
	}

	h.recorder().ApplicationsIngested(ctx, out.hydrated, string(LaneC))

	span.SetAttributes(
		attribute.String("poll.lane", string(LaneC)),
		attribute.Int("poll.records_seen", out.recordsSeen),
		attribute.Int("poll.records_ingested", out.hydrated),
		attribute.Int("reconciliation.authorities_swept", out.authoritiesSwept),
		attribute.Int("reconciliation.stragglers", out.stragglers),
		attribute.String("reconciliation.sample_error_body", out.sampleErrorBody),
		attribute.Int("reconciliation.error_count", out.badRequestCount),
		// tc-mc0hf: lets a prod check confirm the 429 circuit breaker fired
		// by reading this attribute directly (Properties in AppDependencies)
		// instead of cross-referencing raw 429 counts by hand. Absent
		// entirely on the two early-bail-out paths above (authority list
		// load / watermark read failure) along with every other
		// reconciliation.* attribute -- nothing was swept, so there is
		// nothing to report.
		attribute.Bool("reconciliation.rate_limited", out.rateLimited),
	)
	return out
}

// sweepAuthority fetches one authority's light projection page, diffs each
// row against the persisted application, and hydrates the stragglers
// (bounded by MaxStragglersPerAuthority). differentStart is Run's
// once-per-sweep LookbackDays cutoff.
func (h *ReconciliationHandler) sweepAuthority(ctx context.Context, authorityID int, differentStart time.Time, out *reconciliationOutcome) {
	res, err := h.fetcher.FetchReconciliationPage(ctx, authorityID, 0, differentStart)
	if err != nil {
		h.logger.WarnContext(ctx, "lane C: reconciliation page fetch failed, skipping authority", "authorityId", authorityID, "error", err)
		recordFetchError(err, out)
		return
	}
	out.authoritiesSwept++
	out.recordsSeen += len(res.Applications)

	authorityCode := strconv.Itoa(authorityID)
	stragglers := 0
	for _, light := range res.Applications {
		if stragglers >= h.opts.MaxStragglersPerAuthority {
			break
		}
		// A 429 from hydrating an earlier straggler in THIS authority trips
		// the circuit breaker: stop hydrating the rest of this authority's
		// stragglers rather than following a rejected request with more
		// rejected requests (tc-mc0hf).
		if out.rateLimited {
			break
		}
		existing, found, gerr := h.ingester.apps.GetByUID(ctx, light.UID, authorityCode)
		if gerr != nil {
			h.logger.WarnContext(ctx, "lane C: read existing application failed", "uid", light.UID, "error", gerr)
			continue
		}
		if found && !reconciliationDiffers(existing, light) {
			continue
		}
		out.stragglers++
		stragglers++
		h.hydrate(ctx, light.UID, out)
	}
}

// recordFetchError captures a PlanIt fetch error onto the sweep outcome.
// *planit.HTTPError's status/body is captured as before (tc-tuge8/GH#971).
// Independently, *planit.RateLimitError -- a distinct sibling error type,
// never an HTTPError -- trips out.rateLimited (tc-mc0hf), Lane C's circuit
// breaker: Run and sweepAuthority's straggler loop both check it to stop
// making further PlanIt requests for the rest of this cycle. Any other error
// type (e.g. a transport failure) is a no-op here -- it stays logged, as
// before, with nothing to add to the span.
func recordFetchError(err error, out *reconciliationOutcome) {
	var rlErr *planit.RateLimitError
	if errors.As(err, &rlErr) {
		out.rateLimited = true
	}

	var herr *planit.HTTPError
	if !errors.As(err, &herr) {
		return
	}
	if herr.StatusCode == http.StatusBadRequest {
		out.badRequestCount++
	}
	if out.sampleErrorBody == "" && herr.Body != "" {
		out.sampleErrorBody = herr.Body
	}
}

// hydrate fetches one straggler's full record by uid and feeds it through the
// standard Ingester (identical fan-out to Lane A/B). A miss or a hydration
// error is logged and skipped; the next sweep retries it.
func (h *ReconciliationHandler) hydrate(ctx context.Context, uid string, out *reconciliationOutcome) {
	full, err := h.fetcher.FetchByUID(ctx, uid)
	if err != nil {
		h.logger.WarnContext(ctx, "lane C: hydration fetch failed", "uid", uid, "error", err)
		recordFetchError(err, out)
		return
	}
	for _, app := range full.Applications {
		if app.UID != uid {
			continue
		}
		if ierr := h.ingester.Ingest(ctx, app); ierr != nil {
			h.logger.WarnContext(ctx, "lane C: hydrated ingest failed", "uid", uid, "error", ierr)
			return
		}
		out.hydrated++
		return
	}
	h.logger.WarnContext(ctx, "lane C: hydration fetch returned no matching record", "uid", uid)
}

// reconciliationDiffers reports whether the light projection row's app_state,
// decided_date, or last_different differs from the persisted application —
// Lane C's straggler test. Every other field is deliberately ignored: the
// light projection never carries them, so comparing full business fields
// here (applications.PlanningApplication.HasSameBusinessFieldsAs) would flag
// every row as a false-positive straggler.
func reconciliationDiffers(existing, light applications.PlanningApplication) bool {
	if !eqOptionalString(existing.AppState, light.AppState) {
		return true
	}
	if !eqOptionalTime(existing.DecidedDate, light.DecidedDate) {
		return true
	}
	return !existing.LastDifferent.Equal(light.LastDifferent)
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
