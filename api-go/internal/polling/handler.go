// Package polling is the Go port of the .NET TownCrier.Application.Polling and
// TownCrier.Infrastructure.Polling slices: the Service-Bus-triggered adaptive
// PlanIt poll (WORKER_MODE=poll-sb). It owns the trigger orchestrator, the
// PlanIt ingestion handler, the next-run scheduler, the Cosmos etag-CAS lease
// and poll-state stores, and the cycle-alternating authority providers.
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
	"time"

	"github.com/AmyDe/town-crier/api-go/internal/applications"
	"github.com/AmyDe/town-crier/api-go/internal/planit"
)

// resumeOverlap is the one-page overlap applied when resuming from a saved
// cursor (start at max(1, NextPage-1)) to tolerate PlanIt page-shift between
// cycles, matching .NET's resumable-cursor logic.
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

// HandlerOptions are the ingestion tunables. MaxPagesPerAuthorityPerCycle caps
// pagination per authority (nil = unbounded). HandlerBudget is the soft
// wall-clock deadline; zero disables it. Mirrors the relevant .NET PollingOptions
// fields.
type HandlerOptions struct {
	MaxPagesPerAuthorityPerCycle *int
	HandlerBudget                time.Duration
}

// PollPlanItResult is the outcome of one ingestion cycle. Mirrors .NET
// PollPlanItResult and drives the worker's exit code (exit 1 only when
// ApplicationCount==0 AND AuthorityErrors>0) and the next-run cadence.
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
// poll state (high-water mark + resumable cursor). It is the Go port of .NET
// PollPlanItCommandHandler, scoped to ingestion (zone fan-out and decision-event
// dispatch are the notification pipeline's concern and not wired into the worker
// poll path — see bd follow-up).
type PollPlanItHandler struct {
	planIt      planItFetcher
	apps        applicationStore
	state       pollStateAccess
	authorities activeAuthorityProvider
	cycle       cycleSelector
	opts        HandlerOptions
	now         func() time.Time
	logger      *slog.Logger
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

	activeIDs, err := h.authorities.ActiveAuthorityIDs(ctx)
	if err != nil {
		return PollPlanItResult{}, err
	}
	lru, err := h.state.GetLeastRecentlyPolled(ctx, activeIDs)
	if err != nil {
		return PollPlanItResult{}, err
	}

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

		outcome := h.pollAuthority(ctx, authorityID, now, budgetExhausted)
		count += outcome.appCount
		if outcome.rateLimited {
			rateLimited = true
			retryAfter = outcome.retryAfter
		}
		if outcome.timeBounded {
			timeBounded = true
		}
		switch {
		case outcome.err != nil:
			authorityErrors++
			h.logger.ErrorContext(ctx, "error polling authority, skipping to next", "polling.authority_code", authorityID, "error", outcome.err)
		case outcome.completed || outcome.appCount > 0:
			authoritiesPoll++
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
func (h *PollPlanItHandler) pollAuthority(ctx context.Context, authorityID int, now time.Time, budgetExhausted func() bool) authorityOutcome {
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
		}

		for _, app := range res.Applications {
			out.appCount++
			if app.LastDifferent.After(highWaterMark) {
				highWaterMark = app.LastDifferent
			}
			if err := h.upsertIfChanged(ctx, app); err != nil {
				out.err = err
				return h.finishAuthority(ctx, authorityID, now, out, highWaterMark, existingHWM, lastPageFetched, firstPageTotal, capHit, hadCursor)
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
	return h.finishAuthority(ctx, authorityID, now, out, highWaterMark, existingHWM, lastPageFetched, firstPageTotal, capHit, hadCursor)
}

// upsertIfChanged point-reads the persisted application by uid within its
// authority partition and upserts only when a business field changed (the
// reindex-flood guard). A first-time insert (no existing record) always upserts.
func (h *PollPlanItHandler) upsertIfChanged(ctx context.Context, app applications.PlanningApplication) error {
	authorityCode := strconv.Itoa(app.AreaID)
	existing, found, err := h.apps.GetByUID(ctx, app.UID, authorityCode)
	if err != nil {
		return err
	}
	if found && existing.HasSameBusinessFieldsAs(app) {
		return nil
	}
	return h.apps.Upsert(ctx, app)
}

// finishAuthority persists the authority's advanced poll state. On a cap-hit or
// mid-pagination 429 it freezes the HWM and saves a resumable cursor at the next
// unfetched page; on a natural end it advances the HWM to the max LastDifferent
// observed and clears any cursor. State is only written when the authority
// produced work (completed or saw applications), matching .NET.
func (h *PollPlanItHandler) finishAuthority(
	ctx context.Context,
	authorityID int,
	now time.Time,
	out authorityOutcome,
	highWaterMark, existingHWM time.Time,
	lastPageFetched int,
	firstPageTotal *int,
	capHit, hadCursor bool,
) authorityOutcome {
	if !out.completed && out.appCount == 0 {
		return out
	}

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
	_ = hadCursor // telemetry-only in .NET (cursor_cleared counter); omitted here
	return out
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
