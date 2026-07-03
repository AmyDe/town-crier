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
	"github.com/AmyDe/town-crier/api-go/internal/watchzones"
)

// pollAppConsumer mirrors polling's unexported applicationStore: the exact slice
// of applications.Store the poll handler consumes — the change-dedup read and the
// Upsert write the poll cycle performs. The worker hands NewPollPlanItHandler the
// applications.Store interface, so that interface must satisfy this contract for
// the poll Upsert to reach the store.
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

	handler := &polling.PollPlanItHandler{}
	spy := newSpyZoneStore()

	// wirePollFanOut must accept the watchzones.Store interface (the spy) and wire
	// the fan-out onto the handler without panicking on a nil stores pointer.
	wirePollFanOut(platform.Config{}, handler, spy, testRegistry(), nil, discardLogger())
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
