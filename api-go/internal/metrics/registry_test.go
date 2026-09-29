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
	reg.PlanItHTTPError(ctx, 500, 99)
	reg.NotificationCreated(ctx, "NewApplication", "Zone")
	reg.WatchZoneCreated(ctx)
	reg.WatchZoneUpdated(ctx)
	reg.WatchZoneDeleted(ctx)
	reg.PushDeliveryFailed(ctx, "apns")

	got := metricNames(collect())

	want := []string{
		"towncrier.planit.http_errors",
		"towncrier.notifications.created",
		"towncrier.watchzones.created",
		"towncrier.watchzones.updated",
		"towncrier.watchzones.deleted",
		"towncrier.push.delivery_failed",
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
	reg.PlanItHTTPError(ctx, 500, 99)
	reg.NotificationCreated(ctx, "NewApplication", "Zone")
	reg.WatchZoneCreated(ctx)
	reg.WatchZoneUpdated(ctx)
	reg.WatchZoneDeleted(ctx)
	reg.PushDeliveryFailed(ctx, "apns")
}

// TestRegistry_PushDeliveryFailedTagsPlatform pins tc-97k35.4: the counter must
// carry the platform tag ("apns" | "fcm") so a dashboard/alert can distinguish
// which sender is failing, mirroring PlanItHTTPError's status-code tag.
func TestRegistry_PushDeliveryFailedTagsPlatform(t *testing.T) {
	t.Parallel()
	reg, collect := newTestRegistry(t)
	ctx := context.Background()

	reg.PushDeliveryFailed(ctx, "fcm")

	rm := collect()
	var foundPlatform bool
	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			if m.Name != "towncrier.push.delivery_failed" {
				continue
			}
			sum, ok := m.Data.(metricdata.Sum[int64])
			if !ok {
				t.Fatalf("push.delivery_failed is not an int64 sum: %T", m.Data)
			}
			for _, dp := range sum.DataPoints {
				if v, ok := dp.Attributes.Value("platform"); ok {
					foundPlatform = true
					if v.AsString() != "fcm" {
						t.Errorf("platform = %q, want fcm", v.AsString())
					}
				}
			}
		}
	}
	if !foundPlatform {
		t.Error("push.delivery_failed missing platform tag")
	}
}
