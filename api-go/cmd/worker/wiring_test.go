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
// Upsert write the poll cycle performs. The worker hands each ADR 0044 lane
// (NewNationalLaneHandler / NewInverseMaskLaneHandler) the applications.Store
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
	laneC := &polling.InverseMaskLaneHandler{}
	handler := &polling.NationalPollHandler{}
	spy := newSpyZoneStore()

	// wirePollFanOut must accept the watchzones.Store interface (the spy) and wire
	// the fan-out onto all four lanes without panicking on a nil stores pointer.
	wirePollFanOut(platform.Config{}, laneA, laneB, laneC, handler, spy, testRegistry(), nil, discardLogger())
}

// TestWirePollFanOut_NilLaneCDoesNotPanic proves wirePollFanOut's nil guard
// still holds even though production always wires a real Lane C now (ADR
// 0044 dropped POLLING_LANE_C_ENABLED): a test exercising a narrower lane
// set must not need to stub Lane C too. Before the original tc-5lu8h fix,
// wirePollFanOut called laneC.WithFanOut unconditionally, which nil-panics
// on a nil receiver method value dereference.
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

// pollOrchestratorTestConfig returns the minimal platform.Config
// buildPollOrchestrator needs to construct without error: a PlanIt base URL
// and the ADR 0044 day-window fields (LoadConfig's own defaults —
// buildPollOrchestrator now parses these via polling.ParseCivilTime and
// fails the whole build on a malformed value, so a zero-value Config{} is no
// longer a valid input here).
func pollOrchestratorTestConfig() platform.Config {
	return platform.Config{
		PlanItBaseURL:   "https://stub.planit.test/",
		PollingDayStart: "07:00",
		PollingDayEnd:   "19:00",
	}
}

// TestBuildPollOrchestrator_AlwaysWiresLaneC pins ADR 0044's removal of the
// POLLING_LANE_C_ENABLED gate: Lane C (the national inverse-mask
// reconciliation lane) is now constructed and wired unconditionally, with no
// flag to check. sbClient and st are zero-value: buildPollOrchestrator only
// needs sbClient non-nil to pass its "no poller configured" guard, and every
// collaborator it constructs (planit.NewClient, the lane handlers, the
// planner, the orchestrator) opens no connection and performs no I/O at
// construction time.
func TestBuildPollOrchestrator_AlwaysWiresLaneC(t *testing.T) {
	t.Parallel()

	sbClient := &servicebus.Client{}
	st := &stores{}

	adapter, err := buildPollOrchestrator(pollOrchestratorTestConfig(), sbClient, testRegistry(), st, discardLogger())
	if err != nil {
		t.Fatalf("buildPollOrchestrator: %v", err)
	}
	if adapter == nil {
		t.Fatal("buildPollOrchestrator: got nil adapter, want a configured orchestrator")
	}
}

// TestBuildPollOrchestrator_InvalidDayWindowFailsTheBuild proves a malformed
// POLLING_DAY_START/POLLING_DAY_END fails buildPollOrchestrator outright
// (never silently degrades Lane C/D's eligibility window to some default).
func TestBuildPollOrchestrator_InvalidDayWindowFailsTheBuild(t *testing.T) {
	t.Parallel()

	cfg := pollOrchestratorTestConfig()
	cfg.PollingDayStart = "not-a-time"
	sbClient := &servicebus.Client{}
	st := &stores{}

	if _, err := buildPollOrchestrator(cfg, sbClient, testRegistry(), st, discardLogger()); err == nil {
		t.Fatal("buildPollOrchestrator: got nil error, want a parse failure for an invalid POLLING_DAY_START")
	}
}

// TestBuildPollOrchestrator_BackfillGating pins GH#967's dark-ship default:
// Lane D (the paced historical backfill lane) is constructed and wired only
// when cfg.PollingBackfillEnabled is true (default false). Both gate states
// must build a working orchestrator without panicking — every collaborator
// buildPollOrchestrator constructs opens no connection and performs no I/O at
// construction time, so a zero-value *stores works for both branches.
func TestBuildPollOrchestrator_BackfillGating(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		backfillEnabled bool
	}{
		{"disabled (default)", false},
		{"enabled", true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			cfg := pollOrchestratorTestConfig()
			cfg.PollingBackfillEnabled = tc.backfillEnabled
			cfg.PollingBackfillWindowWidthDays = 90
			cfg.PollingBackfillMaxPagesPerCycle = 2
			cfg.PollingBackfillEmptyWindowsBeforeComplete = 12
			sbClient := &servicebus.Client{}
			// Referencing the backfill field by name (even as nil) forces
			// stores to carry a *polling.PostgresBackfillStateStore field —
			// buildPollOrchestrator's real construction path reads it when
			// PollingBackfillEnabled is true.
			st := &stores{backfill: nil}

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
