// Package polling implements the Service-Bus-triggered adaptive PlanIt poll
// (WORKER_MODE=poll-sb). It owns the trigger orchestrator, the PlanIt ingestion
// handler, the next-run scheduler, the Cosmos etag-CAS lease and poll-state
// stores, and the cycle-alternating authority providers.
//
// The crash-safety model is receive-and-delete + publish-after-consume per
// ADR 0024 (2026-04-22 amendment): the orchestrator acquires a Cosmos lease,
// destructively receives one trigger, runs the handler, publishes the next
// scheduled trigger, then releases the lease. There is no Service Bus
// Complete/Abandon — the safety-net bootstrap (internal/worker) is the sole
// recovery path when anything fails between receive and publish.
package polling

import (
	"context"
	"errors"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/AmyDe/town-crier/api-go/internal/applications"
	"github.com/AmyDe/town-crier/api-go/internal/planit"
)

// resumeOverlap is the one-page overlap applied when resuming from a saved
// cursor (start at max(1, NextPage-1)) to tolerate PlanIt page-shift between
// cycles.
const resumeOverlap = 1

// planItFetcher is the consumer-side slice of the PlanIt client the handler
// needs: fetch one page for an authority. *planit.Client satisfies it.
type planItFetcher interface {
	FetchApplicationsPage(ctx context.Context, authorityID int, differentStart *time.Time, page int) (planit.FetchPageResult, error)
}

// applicationStore is the consumer-side slice of the Applications store: a
// partition-scoped existence read and an upsert. *applications.CosmosStore
// satisfies it.
type applicationStore interface {
	GetByUID(ctx context.Context, uid, authorityCode string) (applications.PlanningApplication, bool, error)
	Upsert(ctx context.Context, a applications.PlanningApplication) error
}

// pollStateAccess is the consumer-side slice of the poll-state store the handler
// needs. *PollStateStore satisfies it.
type pollStateAccess interface {
	Get(ctx context.Context, authorityID int) (PollState, bool, error)
	Save(ctx context.Context, authorityID int, lastPollTime, highWaterMark time.Time, cursor *PollCursor) error
	GetLeastRecentlyPolled(ctx context.Context, candidateAuthorityIDs []int) (LeastRecentlyPolledResult, error)
}

// activeAuthorityProvider yields the authorities to walk this cycle.
type activeAuthorityProvider interface {
	ActiveAuthorityIDs(ctx context.Context) ([]int, error)
}

// DecisionDispatcher dispatches a decision-update event for a polled application
// that has just transitioned into a decision state. *notifydispatch.DecisionDispatcher
// satisfies it. It is exported because cmd/worker wires the concrete dispatcher
// behind it.
type DecisionDispatcher interface {
	Dispatch(ctx context.Context, app applications.PlanningApplication) error
}

// NotificationEnqueuer fans a newly-changed application out to the watch zones
// that contain it, enqueuing one notification per eligible zone.
// *notifydispatch.Enqueuer satisfies it via EnqueueForApplication.
type NotificationEnqueuer interface {
	EnqueueForApplication(ctx context.Context, app applications.PlanningApplication) error
}

// metricsRecorder is the consumer-side slice of the metrics registry the handler
// records the per-cycle / per-authority polling KPIs on. *metrics.Registry
// satisfies it; a nil recorder no-ops every call, so the handler records nothing
// until WithMetrics wires one (matching the WithFanOut pattern). cycleType is the
// CycleType.TelemetryValue() string ("Watched" | "Seed") every instrument tags.
type metricsRecorder interface {
	AuthorityPolled(ctx context.Context, cycleType string)
	AuthoritySkipped(ctx context.Context, cycleType string)
	ApplicationsIngested(ctx context.Context, n int, cycleType string)
	RateLimited(ctx context.Context, cycleType string)
	RetryAfterSeconds(ctx context.Context, seconds float64, cycleType string, authorityID int, headerPresent bool)
	AuthorityProcessingMillis(ctx context.Context, ms float64, cycleType string)
	AuthorityTotal(ctx context.Context, total int, cycleType string, authorityID int)
	CycleCompleted(ctx context.Context, cycleType, termination string)
	CursorAdvanced(ctx context.Context, cycleType string)
	CursorCleared(ctx context.Context, cycleType string)
	OldestHighWaterMarkAge(ctx context.Context, seconds float64, cycleType string, authorityID int, neverPolled bool)
	NeverPolledCount(ctx context.Context, count int, cycleType string)
}

// HandlerOptions are the ingestion tunables. MaxPagesPerAuthorityPerCycle caps
// pagination per authority (nil = unbounded). HandlerBudget is the soft
// wall-clock deadline; zero disables it.
type HandlerOptions struct {
	MaxPagesPerAuthorityPerCycle *int
	HandlerBudget                time.Duration
}

// PollPlanItResult is the outcome of one ingestion cycle. Drives the worker's
// exit code (exit 1 only when ApplicationCount==0 AND AuthorityErrors>0) and
// the next-run cadence.
type PollPlanItResult struct {
	ApplicationCount  int
	AuthoritiesPolled int
	RateLimited       bool
	TerminationReason TerminationReason
	AuthorityErrors   int
	// RetryAfter is the Retry-After hint bubbled up from a 429, consumed by the
	// scheduler to time the next trigger. nil when not rate-limited or absent.
	RetryAfter *time.Duration
}

// PollPlanItHandler runs one adaptive PlanIt poll cycle: select the active
// authorities for the current cycle type, walk them least-recently-polled first,
// page each up to the cap, upsert changed applications, and advance per-authority
// poll state (high-water mark + resumable cursor). Scoped to ingestion (zone
// fan-out and decision-event dispatch are the notification pipeline's concern and
// not wired into the worker poll path — see bd follow-up).
type PollPlanItHandler struct {
	planIt      planItFetcher
	apps        applicationStore
	state       pollStateAccess
	authorities activeAuthorityProvider
	cycle       cycleSelector
	opts        HandlerOptions
	now         func() time.Time
	logger      *slog.Logger

	// decision and enqueuer are the optional poll-path notification fan-out
	// collaborators wired via WithFanOut. When nil, the handler ingests only (the
	// behaviour before bead tc-uc2p); when set, each upserted application drives a
	// decision-event dispatch (on a non-decision -> decision transition) and a
	// watch-zone notification fan-out.
	decision DecisionDispatcher
	enqueuer NotificationEnqueuer

	// metrics records the towncrier.polling.* business KPIs, wired via WithMetrics.
	// nil until wired (the no-metrics default), so the handler records nothing
	// during the many ingestion-only tests and call sites that don't supply one.
	metrics metricsRecorder
}

// WithMetrics wires the metrics recorder the handler records the per-cycle and
// per-authority polling KPIs on. Like WithFanOut it is a post-construction setter
// so the ingestion-only call sites and tests are unaffected; cmd/worker calls it
// once after building the handler. Returns the handler for chaining.
func (h *PollPlanItHandler) WithMetrics(rec metricsRecorder) *PollPlanItHandler {
	h.metrics = rec
	return h
}

// recorder returns a non-nil recorder so call sites can record unconditionally.
// When no recorder is wired it returns a no-op so the per-call nil checks live in
// one place rather than at every record site.
func (h *PollPlanItHandler) recorder() metricsRecorder {
	if h.metrics == nil {
		return noopMetrics{}
	}
	return h.metrics
}

// WithFanOut wires the notification fan-out collaborators the worker runs on the
// poll path: the decision-event dispatcher and the watch-zone enqueuer. It is a
// post-construction setter rather than a constructor parameter so the many
// ingestion-only call sites and tests are unaffected; cmd/worker calls it once
// after building the handler. Returns the handler for chaining.
func (h *PollPlanItHandler) WithFanOut(decision DecisionDispatcher, enqueuer NotificationEnqueuer) *PollPlanItHandler {
	h.decision = decision
	h.enqueuer = enqueuer
	return h
}

// NewPollPlanItHandler wires the handler. now is injected so tests pin the clock.
func NewPollPlanItHandler(
	planIt planItFetcher,
	apps applicationStore,
	state pollStateAccess,
	authorities activeAuthorityProvider,
	cycle cycleSelector,
	opts HandlerOptions,
	now func() time.Time,
	logger *slog.Logger,
) *PollPlanItHandler {
	return &PollPlanItHandler{
		planIt:      planIt,
		apps:        apps,
		state:       state,
		authorities: authorities,
		cycle:       cycle,
		opts:        opts,
		now:         now,
		logger:      logger,
	}
}

// CurrentCycle reports the cycle type the next Handle call will run, so the
// dispatch layer can tag the telemetry span with cycle.type before invoking it.
func (h *PollPlanItHandler) CurrentCycle() CycleType {
	return h.cycle.Current()
}

// Handle runs one ingestion cycle and returns its result. It never returns an
// error for an empty active set or per-authority failures (those are counted and
// the cycle continues); it returns an error only when the candidate selection
// itself fails (e.g. the cross-partition LRU query errored).
func (h *PollPlanItHandler) Handle(ctx context.Context) (PollPlanItResult, error) {
	now := h.now().UTC()

	var deadline time.Time
	hasDeadline := h.opts.HandlerBudget > 0
	if hasDeadline {
		deadline = now.Add(h.opts.HandlerBudget)
	}
	budgetExhausted := func() bool {
		return hasDeadline && !h.now().UTC().Before(deadline)
	}

	cycleType := h.cycle.Current().TelemetryValue()
	rec := h.recorder()

	activeIDs, err := h.authorities.ActiveAuthorityIDs(ctx)
	if err != nil {
		return PollPlanItResult{}, err
	}
	lru, err := h.state.GetLeastRecentlyPolled(ctx, activeIDs)
	if err != nil {
		return PollPlanItResult{}, err
	}

	// Cycle-start gauges: the never-polled backlog and the staleness of the
	// oldest authority's LastPollTime, so the lag dashboards keep working.
	rec.NeverPolledCount(ctx, lru.NeverPolledCount, cycleType)
	h.recordOldestHWMAge(ctx, rec, lru, now, cycleType)

	var (
		count           int
		authoritiesPoll int
		authorityErrors int
		rateLimited     bool
		timeBounded     bool
		retryAfter      *time.Duration
	)

	for _, authorityID := range lru.AuthorityIDs {
		// Graceful timeout: stop before starting a new authority when the outer
		// context is cancelled or the soft budget is spent, returning the partial
		// result cleanly.
		if ctx.Err() != nil || budgetExhausted() {
			timeBounded = true
			break
		}

		authorityStart := h.now()
		outcome := h.pollAuthority(ctx, authorityID, now, budgetExhausted, cycleType)
		rec.AuthorityProcessingMillis(ctx, h.now().Sub(authorityStart).Seconds()*1000, cycleType)
		count += outcome.appCount

		if outcome.rateLimited {
			rateLimited = true
			retryAfter = outcome.retryAfter
			rec.RateLimited(ctx, cycleType)
			h.recordRetryAfter(ctx, rec, outcome.retryAfter, cycleType, authorityID)
		}
		if outcome.timeBounded {
			timeBounded = true
		}
		switch {
		case outcome.err != nil:
			authorityErrors++
			rec.AuthoritySkipped(ctx, cycleType)
			h.logger.ErrorContext(ctx, "error polling authority, skipping to next", "authorityCode", authorityID, "error", outcome.err)
		case outcome.completed || outcome.appCount > 0:
			authoritiesPoll++
			rec.AuthorityPolled(ctx, cycleType)
			rec.ApplicationsIngested(ctx, outcome.appCount, cycleType)
		default:
			// Polled but produced no work and did not error: counts as skipped
			// (authorities_skipped on a quiet authority).
			rec.AuthoritySkipped(ctx, cycleType)
		}

		// A 429 stops the whole cycle so the scheduler can back off via Retry-After.
		if outcome.rateLimited {
			break
		}
	}

	reason := TerminationNatural
	switch {
	case rateLimited:
		reason = TerminationRateLimited
	case timeBounded:
		reason = TerminationTimeBounded
	}

	rec.CycleCompleted(ctx, cycleType, reason.TelemetryValue())

	return PollPlanItResult{
		ApplicationCount:  count,
		AuthoritiesPolled: authoritiesPoll,
		RateLimited:       rateLimited,
		TerminationReason: reason,
		AuthorityErrors:   authorityErrors,
		RetryAfter:        retryAfter,
	}, nil
}

// authorityOutcome captures one authority's processing result.
type authorityOutcome struct {
	appCount    int
	completed   bool
	rateLimited bool
	timeBounded bool
	retryAfter  *time.Duration
	err         error
}

// pollAuthority pages one authority from its resume point up to the cap, upserts
// changed applications, and persists the advanced poll state (HWM + cursor). A
// 429 is an expected outcome (not an error); a transport/5xx failure after
// retries is an authority error that the caller counts and skips past.
func (h *PollPlanItHandler) pollAuthority(ctx context.Context, authorityID int, now time.Time, budgetExhausted func() bool, cycleType string) authorityOutcome {
	existing, _, err := h.state.Get(ctx, authorityID)
	existingHWM := now.AddDate(0, 0, -1)
	hadCursor := false
	if err == nil {
		if !existing.HighWaterMark.IsZero() {
			existingHWM = existing.HighWaterMark
		}
		hadCursor = existing.Cursor != nil
	}

	// Resume from a saved cursor only when it still anchors the current HWM date,
	// overlapping by one page to tolerate PlanIt page-shift. Otherwise start at 1.
	startPage := 1
	if existing.Cursor != nil && sameDate(existing.Cursor.DifferentStart, existingHWM) {
		startPage = max(1, existing.Cursor.NextPage-resumeOverlap)
	}

	maxPages := h.opts.MaxPagesPerAuthorityPerCycle
	var (
		out             authorityOutcome
		highWaterMark   time.Time
		lastPageFetched int
		firstPageTotal  *int
		capHit          bool
		pagesFetched    int
		page            = startPage
	)

	ds := existingHWM
	for {
		res, fetchErr := h.planIt.FetchApplicationsPage(ctx, authorityID, &ds, page)
		if fetchErr != nil {
			var rl *planit.RateLimitError
			if errors.As(fetchErr, &rl) {
				out.rateLimited = true
				out.retryAfter = rl.RetryAfter
				break
			}
			out.err = fetchErr
			break
		}

		if pagesFetched == 0 {
			firstPageTotal = res.Total
			if res.Total != nil {
				h.recorder().AuthorityTotal(ctx, *res.Total, cycleType, authorityID)
			}
		}

		for _, app := range res.Applications {
			out.appCount++
			if app.LastDifferent.After(highWaterMark) {
				highWaterMark = app.LastDifferent
			}
			if err := h.processApplication(ctx, app); err != nil {
				out.err = err
				return h.finishAuthority(ctx, authorityID, now, out, highWaterMark, existingHWM, lastPageFetched, firstPageTotal, capHit, hadCursor, cycleType)
			}
		}

		pagesFetched++
		lastPageFetched = page

		if !res.HasMorePages {
			break
		}
		if maxPages != nil && pagesFetched >= *maxPages {
			capHit = true
			break
		}
		if ctx.Err() != nil || budgetExhausted() {
			capHit = true
			out.timeBounded = true
			break
		}
		page++
	}

	if out.err == nil && !out.rateLimited {
		out.completed = true
	}
	return h.finishAuthority(ctx, authorityID, now, out, highWaterMark, existingHWM, lastPageFetched, firstPageTotal, capHit, hadCursor, cycleType)
}

// processApplication point-reads the persisted application by uid within its
// authority partition, and — when a business field changed (the reindex-flood
// guard; a first-time insert always counts as changed) — upserts it, then runs
// the poll-path notification fan-out: a decision-event dispatch when the app has
// just transitioned into a decision state, and the watch-zone notification
// fan-out.
//
// The new-decision check is computed BEFORE the upsert so it compares the
// PERSISTED state, not the incoming one: a non-decision -> decision transition
// (Permitted/Conditions/Rejected/Appealed), including a first-seen already-decided
// application (existing is absent), dispatches exactly one decision event.
// Downstream idempotency (one decision per user/app) makes a re-dispatch harmless,
// but gating on the transition keeps the dispatch count honest. The fan-out
// collaborators are skipped entirely when not wired (ingestion-only mode).
func (h *PollPlanItHandler) processApplication(ctx context.Context, app applications.PlanningApplication) error {
	authorityCode := strconv.Itoa(app.AreaID)
	existing, found, err := h.apps.GetByUID(ctx, app.UID, authorityCode)
	if err != nil {
		return err
	}
	if found && existing.HasSameBusinessFieldsAs(app) {
		return nil
	}

	var existingState *string
	if found {
		existingState = existing.AppState
	}
	isNewDecision := isDecisionState(app.AppState) && !isDecisionState(existingState)

	if err := h.apps.Upsert(ctx, app); err != nil {
		return err
	}

	if isNewDecision && h.decision != nil {
		if err := h.decision.Dispatch(ctx, app); err != nil {
			return err
		}
	}
	if h.enqueuer != nil {
		if err := h.enqueuer.EnqueueForApplication(ctx, app); err != nil {
			return err
		}
	}
	return nil
}

// isDecisionState reports whether a PlanIt app_state is a decision outcome
// (Permitted, Conditions, Rejected, Appealed), case-insensitively. A nil/empty
// state is not a decision.
func isDecisionState(appState *string) bool {
	if appState == nil || *appState == "" {
		return false
	}
	switch {
	case strings.EqualFold(*appState, "Permitted"),
		strings.EqualFold(*appState, "Conditions"),
		strings.EqualFold(*appState, "Rejected"),
		strings.EqualFold(*appState, "Appealed"):
		return true
	default:
		return false
	}
}

// finishAuthority persists the authority's advanced poll state. On a cap-hit or
// mid-pagination 429 it freezes the HWM and saves a resumable cursor at the next
// unfetched page; on a natural end it advances the HWM to the max LastDifferent
// observed and clears any cursor. State is only written when the authority
// produced work (completed or saw applications).
func (h *PollPlanItHandler) finishAuthority(
	ctx context.Context,
	authorityID int,
	now time.Time,
	out authorityOutcome,
	highWaterMark, existingHWM time.Time,
	lastPageFetched int,
	firstPageTotal *int,
	capHit, hadCursor bool,
	cycleType string,
) authorityOutcome {
	if !out.completed && out.appCount == 0 {
		return out
	}

	rec := h.recorder()

	if capHit || (out.rateLimited && out.appCount > 0) {
		// Freeze HWM, save a resumable cursor at the next unfetched page so the
		// following cycle resumes there. LastPollTime still advances so the
		// scheduler rotates off this authority.
		nextPage := lastPageFetched + 1
		cursor := &PollCursor{
			DifferentStart: truncateToDate(existingHWM),
			NextPage:       nextPage,
			KnownTotal:     firstPageTotal,
		}
		if err := h.state.Save(ctx, authorityID, now, existingHWM, cursor); err != nil && out.err == nil {
			out.err = err
		}
		// A persisted non-null cursor advances the resume point.
		rec.CursorAdvanced(ctx, cycleType)
		return out
	}

	// Natural end: advance HWM to the max LastDifferent observed, falling back to
	// the existing HWM when the authority was quiet. Clear any active cursor.
	advancedHWM := existingHWM
	if !highWaterMark.IsZero() {
		advancedHWM = highWaterMark
	}
	if err := h.state.Save(ctx, authorityID, now, advancedHWM, nil); err != nil && out.err == nil {
		out.err = err
	}
	// Clearing a previously-active cursor at a natural end (cursor_cleared
	// is recorded only when a cursor was actually present).
	if hadCursor {
		rec.CursorCleared(ctx, cycleType)
	}
	return out
}

// recordOldestHWMAge records the staleness of the oldest candidate authority's
// LastPollTime at the start of a cycle. The LRU result orders never-polled-first
// then ascending LastPollTime, so AuthorityIDs[0] is the stalest authority. A
// never-polled authority (no PollState) reports its age from the Unix epoch and
// is tagged never_polled=true so dashboards distinguish it from a genuinely stale
// HWM. An empty candidate set records nothing.
func (h *PollPlanItHandler) recordOldestHWMAge(ctx context.Context, rec metricsRecorder, lru LeastRecentlyPolledResult, now time.Time, cycleType string) {
	if len(lru.AuthorityIDs) == 0 {
		return
	}
	oldestID := lru.AuthorityIDs[0]
	state, found, err := h.state.Get(ctx, oldestID)
	neverPolled := err != nil || !found || state.LastPollTime.IsZero()

	var ageSeconds float64
	if neverPolled {
		ageSeconds = now.Sub(time.Unix(0, 0).UTC()).Seconds()
	} else {
		ageSeconds = now.Sub(state.LastPollTime).Seconds()
	}
	rec.OldestHighWaterMarkAge(ctx, ageSeconds, cycleType, oldestID, neverPolled)
}

// recordRetryAfter records the parsed Retry-After value (seconds) for a 429,
// tagging header_present so dashboards distinguish a PlanIt 429 with no
// Retry-After header (value 0, header_present=false) from a real small backoff.
func (h *PollPlanItHandler) recordRetryAfter(ctx context.Context, rec metricsRecorder, retryAfter *time.Duration, cycleType string, authorityID int) {
	if retryAfter == nil {
		rec.RetryAfterSeconds(ctx, 0, cycleType, authorityID, false)
		return
	}
	rec.RetryAfterSeconds(ctx, retryAfter.Seconds(), cycleType, authorityID, true)
}

// sameDate reports whether two instants fall on the same UTC calendar day.
func sameDate(a, b time.Time) bool {
	ay, am, ad := a.UTC().Date()
	by, bm, bd := b.UTC().Date()
	return ay == by && am == bm && ad == bd
}

// truncateToDate returns the UTC midnight of t's calendar day — the cursor's
// different_start anchor.
func truncateToDate(t time.Time) time.Time {
	y, m, d := t.UTC().Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

// noopMetrics is the recorder used when no metrics registry is wired: every
// method is a no-op so the handler records nothing without per-site nil checks.
type noopMetrics struct{}

func (noopMetrics) AuthorityPolled(context.Context, string)                            {}
func (noopMetrics) AuthoritySkipped(context.Context, string)                           {}
func (noopMetrics) ApplicationsIngested(context.Context, int, string)                  {}
func (noopMetrics) RateLimited(context.Context, string)                                {}
func (noopMetrics) RetryAfterSeconds(context.Context, float64, string, int, bool)      {}
func (noopMetrics) AuthorityProcessingMillis(context.Context, float64, string)         {}
func (noopMetrics) AuthorityTotal(context.Context, int, string, int)                   {}
func (noopMetrics) CycleCompleted(context.Context, string, string)                     {}
func (noopMetrics) CursorAdvanced(context.Context, string)                             {}
func (noopMetrics) CursorCleared(context.Context, string)                              {}
func (noopMetrics) OldestHighWaterMarkAge(context.Context, float64, string, int, bool) {}
func (noopMetrics) NeverPolledCount(context.Context, int, string)                      {}
