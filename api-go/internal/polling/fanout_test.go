package polling

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/AmyDe/town-crier/api-go/internal/applications"
)

// fakeDecisionDispatcher records the applications it was asked to dispatch a
// decision event for.
type fakeDecisionDispatcher struct {
	mu         sync.Mutex
	dispatched []applications.PlanningApplication
}

func (f *fakeDecisionDispatcher) Dispatch(_ context.Context, app applications.PlanningApplication) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.dispatched = append(f.dispatched, app)
	return nil
}

func (f *fakeDecisionDispatcher) count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.dispatched)
}

// fakeEnqueuer records the applications it was asked to fan out to watch zones.
type fakeEnqueuer struct {
	mu       sync.Mutex
	enqueued []applications.PlanningApplication
}

func (f *fakeEnqueuer) EnqueueForApplication(_ context.Context, app applications.PlanningApplication) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.enqueued = append(f.enqueued, app)
	return nil
}

func (f *fakeEnqueuer) count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.enqueued)
}

func decisionApp(name string, areaID int, appState string, lastDifferent time.Time) applications.PlanningApplication {
	s := appState
	return applications.PlanningApplication{
		Name:          name,
		UID:           name + "/FUL",
		AreaName:      "Area",
		AreaID:        areaID,
		Address:       "1 St",
		Description:   "d",
		AppState:      &s,
		LastDifferent: lastDifferent,
	}
}

func TestHandler_FanOut_EnqueuesEveryUpsertedApplication(t *testing.T) {
	t.Parallel()
	pi := newFakePlanIt()
	ld := time.Date(2026, 6, 13, 9, 0, 0, 0, time.UTC)
	pi.pages[pageKey{99, 1}] = planitPage(decisionApp("24/0001", 99, "Undecided", ld))
	apps := newFakeApps()
	state := newFakeStateStore()
	disp := &fakeDecisionDispatcher{}
	enq := &fakeEnqueuer{}
	h := newHandlerWithFanOut(t, pi, apps, state, fakeAuthorities{ids: []int{99}}, CycleSeed, defaultHandlerOpts(), disp, enq)

	if _, err := h.Handle(context.Background()); err != nil {
		t.Fatalf("Handle: %v", err)
	}
	if enq.count() != 1 {
		t.Errorf("every upserted application must be enqueued for zone fan-out, got %d", enq.count())
	}
}

func TestHandler_FanOut_DispatchesDecisionOnTransitionExactlyOnce(t *testing.T) {
	t.Parallel()
	pi := newFakePlanIt()
	ld := time.Date(2026, 6, 13, 9, 0, 0, 0, time.UTC)
	// Incoming app is in a decision state (Permitted).
	pi.pages[pageKey{99, 1}] = planitPage(decisionApp("24/0001", 99, "Permitted", ld))
	apps := newFakeApps()
	// Existing record is NOT in a decision state — so this is a new-decision transition.
	existing := decisionApp("24/0001", 99, "Undecided", ld.Add(-time.Hour))
	apps.existing["24/0001/FUL"] = existing
	state := newFakeStateStore()
	disp := &fakeDecisionDispatcher{}
	enq := &fakeEnqueuer{}
	h := newHandlerWithFanOut(t, pi, apps, state, fakeAuthorities{ids: []int{99}}, CycleSeed, defaultHandlerOpts(), disp, enq)

	if _, err := h.Handle(context.Background()); err != nil {
		t.Fatalf("Handle: %v", err)
	}
	if disp.count() != 1 {
		t.Errorf("non-decision -> decision transition must dispatch exactly once, got %d", disp.count())
	}
}

func TestHandler_FanOut_NoDecisionDispatchWhenAlreadyDecided(t *testing.T) {
	t.Parallel()
	pi := newFakePlanIt()
	ld := time.Date(2026, 6, 13, 9, 0, 0, 0, time.UTC)
	// Incoming app is Conditions (a decision state) but the existing record was
	// ALREADY in a decision state (Permitted) — no transition, so no dispatch.
	pi.pages[pageKey{99, 1}] = planitPage(decisionApp("24/0001", 99, "Conditions", ld))
	apps := newFakeApps()
	apps.existing["24/0001/FUL"] = decisionApp("24/0001", 99, "Permitted", ld.Add(-time.Hour))
	state := newFakeStateStore()
	disp := &fakeDecisionDispatcher{}
	enq := &fakeEnqueuer{}
	h := newHandlerWithFanOut(t, pi, apps, state, fakeAuthorities{ids: []int{99}}, CycleSeed, defaultHandlerOpts(), disp, enq)

	if _, err := h.Handle(context.Background()); err != nil {
		t.Fatalf("Handle: %v", err)
	}
	if disp.count() != 0 {
		t.Errorf("decision->decision change is not a new transition, got %d dispatches", disp.count())
	}
	// It is still a business-field change, so it is upserted and enqueued.
	if enq.count() != 1 {
		t.Errorf("the changed application should still be enqueued, got %d", enq.count())
	}
}

func TestHandler_FanOut_FirstSeenDecidedAppCountsAsTransition(t *testing.T) {
	t.Parallel()
	pi := newFakePlanIt()
	ld := time.Date(2026, 6, 13, 9, 0, 0, 0, time.UTC)
	// First-time insert that arrives already decided (no existing record) counts
	// as a transition, mirroring .NET (existing == null).
	pi.pages[pageKey{99, 1}] = planitPage(decisionApp("24/0001", 99, "Rejected", ld))
	apps := newFakeApps()
	state := newFakeStateStore()
	disp := &fakeDecisionDispatcher{}
	enq := &fakeEnqueuer{}
	h := newHandlerWithFanOut(t, pi, apps, state, fakeAuthorities{ids: []int{99}}, CycleSeed, defaultHandlerOpts(), disp, enq)

	if _, err := h.Handle(context.Background()); err != nil {
		t.Fatalf("Handle: %v", err)
	}
	if disp.count() != 1 {
		t.Errorf("a first-seen already-decided app counts as a transition, got %d", disp.count())
	}
}

func TestHandler_FanOut_SkipsUnchangedReindex(t *testing.T) {
	t.Parallel()
	pi := newFakePlanIt()
	ld := time.Date(2026, 6, 13, 9, 0, 0, 0, time.UTC)
	app := decisionApp("24/0001", 99, "Permitted", ld)
	pi.pages[pageKey{99, 1}] = planitPage(app)
	apps := newFakeApps()
	// Identical business fields already stored (only LastDifferent differs) —
	// the reindex-flood guard must skip both upsert AND fan-out.
	existing := app
	existing.LastDifferent = ld.Add(-time.Hour)
	apps.existing["24/0001/FUL"] = existing
	state := newFakeStateStore()
	disp := &fakeDecisionDispatcher{}
	enq := &fakeEnqueuer{}
	h := newHandlerWithFanOut(t, pi, apps, state, fakeAuthorities{ids: []int{99}}, CycleSeed, defaultHandlerOpts(), disp, enq)

	if _, err := h.Handle(context.Background()); err != nil {
		t.Fatalf("Handle: %v", err)
	}
	if len(apps.upserts) != 0 {
		t.Errorf("unchanged reindex must not upsert, got %d", len(apps.upserts))
	}
	if disp.count() != 0 || enq.count() != 0 {
		t.Errorf("unchanged reindex must not dispatch or enqueue: dispatch=%d enqueue=%d", disp.count(), enq.count())
	}
}
