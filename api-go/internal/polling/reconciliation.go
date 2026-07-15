package polling

import (
	"context"
	"fmt"
	"log/slog"
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
	FetchReconciliationPage(ctx context.Context, authorityID, startIndex int) (planit.FetchPageResult, error)
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

// Due reports whether Lane C's sweep interval has elapsed since its last run
// (or it has never run). A watermark-store read error fails safe (not due):
// skipping a sweep this cycle is far cheaper than risking an unbounded extra
// PlanIt load off the back of a store hiccup.
func (h *ReconciliationHandler) Due(ctx context.Context, now time.Time) bool {
	_, lastRun, err := h.watermark.get(ctx)
	if err != nil {
		h.logger.WarnContext(ctx, "lane C: read last-run watermark failed; skipping this cycle's due check", "error", err)
		return false
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
}

// Run sweeps every pollable authority's light projection, diffs each row
// against Postgres, and hydrates genuinely-differing rows one uid at a time
// (bulk hydration by id_match is unproven — ADR 0041). A per-authority fetch
// error is logged and skipped, not fatal to the sweep: the next scheduled
// sweep retries it.
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

	for _, authorityID := range ids {
		if ctx.Err() != nil {
			break
		}
		h.sweepAuthority(ctx, authorityID, &out)
	}

	// Lane C's watermark row carries only a last-run timestamp (the Due gate)
	// — it has no delta boundary of its own, so the watermark value itself is
	// always the zero time.
	if serr := h.watermark.save(ctx, now, time.Time{}); serr != nil && out.err == nil {
		out.err = serr
	}

	h.recorder().ApplicationsIngested(ctx, out.hydrated, string(LaneC))

	span.SetAttributes(
		attribute.String("poll.lane", string(LaneC)),
		attribute.Int("poll.records_seen", out.recordsSeen),
		attribute.Int("poll.records_ingested", out.hydrated),
		attribute.Int("reconciliation.authorities_swept", out.authoritiesSwept),
		attribute.Int("reconciliation.stragglers", out.stragglers),
	)
	return out
}

// sweepAuthority fetches one authority's light projection page, diffs each
// row against the persisted application, and hydrates the stragglers
// (bounded by MaxStragglersPerAuthority).
func (h *ReconciliationHandler) sweepAuthority(ctx context.Context, authorityID int, out *reconciliationOutcome) {
	res, err := h.fetcher.FetchReconciliationPage(ctx, authorityID, 0)
	if err != nil {
		h.logger.WarnContext(ctx, "lane C: reconciliation page fetch failed, skipping authority", "authorityId", authorityID, "error", err)
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

// hydrate fetches one straggler's full record by uid and feeds it through the
// standard Ingester (identical fan-out to Lane A/B). A miss or a hydration
// error is logged and skipped; the next sweep retries it.
func (h *ReconciliationHandler) hydrate(ctx context.Context, uid string, out *reconciliationOutcome) {
	full, err := h.fetcher.FetchByUID(ctx, uid)
	if err != nil {
		h.logger.WarnContext(ctx, "lane C: hydration fetch failed", "uid", uid, "error", err)
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
