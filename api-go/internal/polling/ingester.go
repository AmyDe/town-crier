package polling

import (
	"context"
	"strconv"
	"strings"

	"github.com/AmyDe/town-crier/api-go/internal/applications"
)

// Ingester ingests a single planning application: upsert (with the reindex-flood
// dedup guard), decision-transition detection, decision-event dispatch, and
// watch-zone notification fan-out. It is exported so callers other than the
// PlanIt poll cycle can feed it one application at a time without pulling in the
// fetch/state/authority/cycle machinery PollPlanItHandler needs to walk PlanIt
// itself — e.g. the dev-seed job (bd tc-grvu.4), which sources applications from
// a read-only prod mirror instead of PlanIt.
type Ingester struct {
	apps     applicationStore
	decision DecisionDispatcher   // may be nil: ingestion-only mode skips decision dispatch
	enqueuer NotificationEnqueuer // may be nil: ingestion-only mode skips zone fan-out
}

// NewIngester wires an Ingester. decision and enqueuer may be nil for
// ingestion-only callers that don't want notification fan-out.
func NewIngester(apps applicationStore, decision DecisionDispatcher, enqueuer NotificationEnqueuer) *Ingester {
	return &Ingester{apps: apps, decision: decision, enqueuer: enqueuer}
}

// Ingest point-reads the persisted application by uid within its authority
// partition, and — when a business field changed (the reindex-flood guard; a
// first-time insert always counts as changed) — upserts it, then runs the
// notification fan-out: a decision-event dispatch when the app has just
// transitioned into a decision state, and the watch-zone notification fan-out.
//
// The new-decision check is computed BEFORE the upsert so it compares the
// PERSISTED state, not the incoming one: a non-decision -> decision transition
// (Permitted/Conditions/Rejected/Appealed), including a first-seen already-decided
// application (existing is absent), dispatches exactly one decision event.
// Downstream idempotency (one decision per user/app) makes a re-dispatch harmless,
// but gating on the transition keeps the dispatch count honest. The fan-out
// collaborators are skipped entirely when not wired (ingestion-only mode).
func (i *Ingester) Ingest(ctx context.Context, app applications.PlanningApplication) error {
	authorityCode := strconv.Itoa(app.AreaID)
	existing, found, err := i.apps.GetByUID(ctx, app.UID, authorityCode)
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

	if err := i.apps.Upsert(ctx, app); err != nil {
		return err
	}

	if isNewDecision && i.decision != nil {
		if err := i.decision.Dispatch(ctx, app); err != nil {
			return err
		}
	}
	if i.enqueuer != nil {
		if err := i.enqueuer.EnqueueForApplication(ctx, app); err != nil {
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
