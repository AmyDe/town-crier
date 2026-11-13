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
	"github.com/AmyDe/town-crier/api-go/internal/watchzones"
)

// notifyZoneConsumer mirrors notifydispatch's unexported zoneMatcher; the
// assertion below proves watchzones.Store satisfies it.
type notifyZoneConsumer interface {
	FindZonesContaining(ctx context.Context, latitude, longitude float64) ([]watchzones.WatchZone, error)
}

var _ notifyZoneConsumer = (watchzones.Store)(nil)

func discardLogger() *slog.Logger {
	return slog.New(slog.DiscardHandler)
}

func testRegistry() *metrics.Registry {
	return metrics.New(otel.Meter(metrics.MeterName))
}

// TestEnqueuer_FindZonesContainingFlowsThroughInterface proves the notify fan-out
// reaches the watch-zone store only through watchzones.Store.FindZonesContaining.
// A zero-zone result keeps the downstream collaborators out of the test.
func TestEnqueuer_FindZonesContainingFlowsThroughInterface(t *testing.T) {
	t.Parallel()

	spy := newSpyZoneStore()
	matcher := &fakeDescriptionMatcher{}
	enqueuer := notifydispatch.NewEnqueuer(
		nil, spy, nil, nil, matcher,
		func() string { return "id" },
		func() time.Time { return time.Unix(0, 0).UTC() },
		discardLogger(),
	)

	lat, lng := 51.501, -0.142
	app := applications.PlanningApplication{Latitude: &lat, Longitude: &lng}

	if err := enqueuer.EnqueueForApplication(context.Background(), app, app.LastDifferent); err != nil {
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
