package main

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"go.opentelemetry.io/otel"

	"github.com/AmyDe/town-crier/api-go/internal/applications"
	"github.com/AmyDe/town-crier/api-go/internal/metrics"
	"github.com/AmyDe/town-crier/api-go/internal/notifydispatch"
	"github.com/AmyDe/town-crier/api-go/internal/platform"
	"github.com/AmyDe/town-crier/api-go/internal/polling"
	"github.com/AmyDe/town-crier/api-go/internal/servicebus"
	"github.com/AmyDe/town-crier/api-go/internal/watchzones"
)

// pollAppConsumer mirrors polling's unexported applicationStore: the exact slice
// of applications.Store the poll handler consumes — the change-dedup read and the
// Upsert write the poll cycle performs. The worker hands each ADR 0041 lane
// (NewNationalLaneHandler / NewReconciliationHandler) the applications.Store
// interface, so that interface must satisfy this contract for the poll Upsert
// to reach the store.
type pollAppConsumer interface {
	GetByUID(ctx context.Context, uid, authorityCode string) (applications.PlanningApplication, bool, error)
	Upsert(ctx context.Context, a applications.PlanningApplication) error
}

// notifyZoneConsumer mirrors notifydispatch's unexported zoneMatcher: the notify
// fan-out's only watch-zone dependency, the FindZonesContaining containment
// lookup. The worker threads the watchzones.Store into the fan-out, so the
// interface must satisfy this — the proof that FindZonesContaining reaches the
// store.
type notifyZoneConsumer interface {
	FindZonesContaining(ctx context.Context, latitude, longitude float64) ([]watchzones.WatchZone, error)
}

// Compile-time proof the consumer-side interfaces satisfy what the worker's poll
// Upsert and notify fan-out actually consume.
var (
	_ pollAppConsumer    = (applications.Store)(nil)
	_ notifyZoneConsumer = (watchzones.Store)(nil)
)

func discardLogger() *slog.Logger {
	return slog.New(slog.DiscardHandler)
}

func testRegistry() *metrics.Registry {
	return metrics.New(otel.Meter(metrics.MeterName))
}

// TestWirePollFanOut_AcceptsZoneStoreInterface is the notify-path guard for the
// riskiest wiring change (issue #664 Phase B): wirePollFanOut must take the
// watchzones.Store interface so the poll fan-out's FindZonesContaining runs
// against the Postgres store. The test passes a watchzones.Store spy — it
// compiles only once the parameter is the interface. A nil stores pointer leaves
// every other collaborator unset (the wiring is nil-tolerant), so the wiring runs
// with no other store dependency.
func TestWirePollFanOut_AcceptsZoneStoreInterface(t *testing.T) {
	t.Parallel()

	laneA := &polling.NationalLaneHandler{}
	laneB := &polling.NationalLaneHandler{}
	laneC := &polling.ReconciliationHandler{}
	handler := &polling.NationalPollHandler{}
	spy := newSpyZoneStore()

	// wirePollFanOut must accept the watchzones.Store interface (the spy) and wire
	// the fan-out onto all three lanes without panicking on a nil stores pointer.
	wirePollFanOut(platform.Config{}, laneA, laneB, laneC, handler, spy, testRegistry(), nil, discardLogger())
}

// TestWirePollFanOut_NilLaneCDoesNotPanic pins the tc-5lu8h hotfix: Lane C
// reconciliation is disabled by default (POLLING_LANE_C_ENABLED=false), so
// buildPollOrchestrator passes a nil *polling.ReconciliationHandler as laneC.
// Before the fix, wirePollFanOut called laneC.WithFanOut unconditionally,
// which nil-panics on a nil receiver method value dereference. This proves
// the guard: a nil laneC must not panic, and Lane A/B fan-out wiring must be
// unaffected.
func TestWirePollFanOut_NilLaneCDoesNotPanic(t *testing.T) {
	t.Parallel()

	laneA := &polling.NationalLaneHandler{}
	laneB := &polling.NationalLaneHandler{}
	handler := &polling.NationalPollHandler{}
	spy := newSpyZoneStore()

	wirePollFanOut(platform.Config{}, laneA, laneB, nil, handler, spy, testRegistry(), nil, discardLogger())
}

// TestEnqueuer_FindZonesContainingFlowsThroughInterface proves the notify fan-out
// reaches the watch-zone store solely through the watchzones.Store interface:
// EnqueueForApplication's entry point is FindZonesContaining, so whichever backend
// the flag selects is consumed identically. It mirrors the enqueuer wirePollFanOut
// builds. A zero-zone result keeps the test scoped to the containment lookup — the
// downstream collaborators are never reached.
func TestEnqueuer_FindZonesContainingFlowsThroughInterface(t *testing.T) {
	t.Parallel()

	spy := newSpyZoneStore()
	enqueuer := notifydispatch.NewEnqueuer(
		nil, spy, nil, nil,
		func() string { return "id" },
		func() time.Time { return time.Unix(0, 0).UTC() },
		discardLogger(),
	)

	lat, lng := 51.501, -0.142
	app := applications.PlanningApplication{Latitude: &lat, Longitude: &lng}

	if err := enqueuer.EnqueueForApplication(context.Background(), app); err != nil {
		t.Fatalf("EnqueueForApplication: %v", err)
	}

	if spy.findCalls != 1 {
		t.Fatalf("FindZonesContaining calls = %d, want 1", spy.findCalls)
	}
	if spy.lastFindLat != lat || spy.lastFindLng != lng {
		t.Fatalf("FindZonesContaining coords = (%v, %v), want (%v, %v)",
			spy.lastFindLat, spy.lastFindLng, lat, lng)
	}
}

// TestBuildPollOrchestrator_LaneCGating pins the tc-5lu8h hotfix: Lane C
// reconciliation is wired only when cfg.PollingLaneCEnabled is true (default
// false). Both gate states must build a working orchestrator without
// panicking — disabled exercises the nil-laneC path through
// wirePollFanOut's guard; enabled exercises the still-functional (if
// currently broken upstream, tracked separately as tc-tuge8) construction
// path so the flag itself introduces no regression when flipped back on.
// sbClient and st are zero-value: buildPollOrchestrator only needs sbClient
// non-nil to pass its "no poller configured" guard, and every collaborator
// it constructs (planit.NewClient, the national lane handlers, the
// reconciliation handler, the orchestrator) opens no connection and performs
// no I/O at construction time.
func TestBuildPollOrchestrator_LaneCGating(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		laneCEnabled bool
	}{
		{"disabled (default)", false},
		{"enabled", true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			cfg := platform.Config{
				PlanItBaseURL:                         "https://stub.planit.test/",
				PollingLaneCEnabled:                   tc.laneCEnabled,
				PollingLaneCIntervalHours:             168,
				PollingLaneCMaxStragglersPerAuthority: 10,
			}
			sbClient := &servicebus.Client{}
			st := &stores{}

			adapter, err := buildPollOrchestrator(cfg, sbClient, testRegistry(), st, discardLogger())
			if err != nil {
				t.Fatalf("buildPollOrchestrator: %v", err)
			}
			if adapter == nil {
				t.Fatal("buildPollOrchestrator: got nil adapter, want a configured orchestrator")
			}
		})
	}
}

// TestLeaseTTLFor_AddsFiveMinuteMargin pins the honest-TTL derivation (GH#938
// PR1): the polling lease TTL is the handler budget plus a 5-minute margin,
// covering soft-budget overshoot (the budget is checked between authorities, not
// preemptive) plus container cold-start before the lease is even acquired. The
// previous +30s margin was observed to be too tight: a cycle overran to ~4.9m
// against a 4.5m TTL, letting a peer take over mid-cycle and fork the trigger
// chain.
func TestLeaseTTLFor_AddsFiveMinuteMargin(t *testing.T) {
	t.Parallel()
	got := leaseTTLFor(240 * time.Second) // PollingHandlerBudgetSeconds default
	want := 240*time.Second + 5*time.Minute
	if got != want {
		t.Errorf("leaseTTLFor(240s): got %v, want %v", got, want)
	}
}
