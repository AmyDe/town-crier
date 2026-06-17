package metrics

import (
	"context"
	"testing"

	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
)

// newTestRegistry builds a Registry backed by an in-memory manual reader so a
// test can collect the recorded measurements. It returns the registry and a
// collect func that snapshots the current metrics.
func newTestRegistry(t *testing.T) (*Registry, func() metricdata.ResourceMetrics) {
	t.Helper()
	reader := sdkmetric.NewManualReader()
	mp := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	t.Cleanup(func() {
		_ = mp.Shutdown(context.Background())
	})
	reg := New(mp.Meter("towncrier"))
	collect := func() metricdata.ResourceMetrics {
		var rm metricdata.ResourceMetrics
		if err := reader.Collect(context.Background(), &rm); err != nil {
			t.Fatalf("collect: %v", err)
		}
		return rm
	}
	return reg, collect
}

// metricNames flattens every instrument name present in the collected metrics.
func metricNames(rm metricdata.ResourceMetrics) map[string]struct{} {
	names := map[string]struct{}{}
	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			names[m.Name] = struct{}{}
		}
	}
	return names
}

func TestRegistry_RecordsAllInstrumentNames(t *testing.T) {
	t.Parallel()
	reg, collect := newTestRegistry(t)
	ctx := context.Background()

	// Drive every recording method once so each instrument emits at least one
	// measurement and therefore appears in the collected metrics.
	reg.AuthorityPolled(ctx, "Watched")
	reg.AuthoritySkipped(ctx, "Watched")
	reg.ApplicationsIngested(ctx, 3, "Watched")
	reg.RateLimited(ctx, "Watched")
	reg.RetryAfterSeconds(ctx, 12, "Watched", 7, true)
	reg.AuthorityProcessingMillis(ctx, 42.0, "Watched")
	reg.AuthorityTotal(ctx, 99, "Watched", 7)
	reg.CycleCompleted(ctx, "Watched", "Natural")
	reg.CursorAdvanced(ctx, "Watched")
	reg.CursorCleared(ctx, "Watched")
	reg.LeaseAcquired(ctx, "orchestrator")
	reg.OldestHighWaterMarkAge(ctx, 3600, "Watched", 7, false)
	reg.NeverPolledCount(ctx, 5, "Watched")
	reg.PlanItHTTPError(ctx, 500, 99)
	reg.NotificationCreated(ctx, "NewApplication", "Zone")
	reg.CosmosRequestCharge(ctx, 2.5, "ReadItem", "WatchZones")
	reg.WatchZoneCreated(ctx)
	reg.WatchZoneUpdated(ctx)
	reg.WatchZoneDeleted(ctx)

	got := metricNames(collect())

	want := []string{
		"towncrier.polling.authorities_polled",
		"towncrier.polling.authorities_skipped",
		"towncrier.polling.applications_ingested",
		"towncrier.polling.rate_limited",
		"towncrier.polling.retry_after_seconds",
		"towncrier.polling.authority_processing_ms",
		"towncrier.polling.authority_total",
		"towncrier.polling.cycles_completed",
		"towncrier.polling.cursor_advanced",
		"towncrier.polling.cursor_cleared",
		"towncrier.polling.lease.acquired",
		"towncrier.polling.oldest_hwm_age_seconds",
		"towncrier.polling.never_polled_count",
		"towncrier.planit.http_errors",
		"towncrier.notifications.created",
		"towncrier.cosmos.request_charge_ru",
		"towncrier.watchzones.created",
		"towncrier.watchzones.updated",
		"towncrier.watchzones.deleted",
	}
	for _, name := range want {
		if _, ok := got[name]; !ok {
			t.Errorf("instrument %q not recorded; got %v", name, got)
		}
	}
}

func TestRegistry_NilIsNoOp(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	var reg *Registry // nil

	// Every recording method must be safe to call on a nil registry so call sites
	// can stay metric-agnostic when telemetry is unconfigured.
	reg.AuthorityPolled(ctx, "Watched")
	reg.AuthoritySkipped(ctx, "Watched")
	reg.ApplicationsIngested(ctx, 1, "Watched")
	reg.RateLimited(ctx, "Watched")
	reg.RetryAfterSeconds(ctx, 1, "Watched", 1, false)
	reg.AuthorityProcessingMillis(ctx, 1, "Watched")
	reg.AuthorityTotal(ctx, 1, "Watched", 1)
	reg.CycleCompleted(ctx, "Watched", "Natural")
	reg.CursorAdvanced(ctx, "Watched")
	reg.CursorCleared(ctx, "Watched")
	reg.LeaseAcquired(ctx, "orchestrator")
	reg.OldestHighWaterMarkAge(ctx, 1, "Watched", 1, false)
	reg.NeverPolledCount(ctx, 1, "Watched")
	reg.PlanItHTTPError(ctx, 500, 99)
	reg.NotificationCreated(ctx, "NewApplication", "Zone")
	reg.CosmosRequestCharge(ctx, 1, "ReadItem", "WatchZones")
	reg.WatchZoneCreated(ctx)
	reg.WatchZoneUpdated(ctx)
	reg.WatchZoneDeleted(ctx)
}

func TestRegistry_RetryAfterTagsHeaderPresence(t *testing.T) {
	t.Parallel()
	reg, collect := newTestRegistry(t)
	ctx := context.Background()

	reg.RetryAfterSeconds(ctx, 0, "Seed", 7, false)

	rm := collect()
	var foundHeaderPresent bool
	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			if m.Name != "towncrier.polling.retry_after_seconds" {
				continue
			}
			hist, ok := m.Data.(metricdata.Histogram[float64])
			if !ok {
				t.Fatalf("retry_after_seconds is not a float64 histogram: %T", m.Data)
			}
			for _, dp := range hist.DataPoints {
				if v, ok := dp.Attributes.Value("header_present"); ok {
					foundHeaderPresent = true
					if v.AsString() != "false" {
						t.Errorf("header_present = %q, want false", v.AsString())
					}
				}
			}
		}
	}
	if !foundHeaderPresent {
		t.Error("retry_after_seconds missing header_present tag")
	}
}
