// Package metrics holds the Town Crier business-metric registry: the
// towncrier.* OpenTelemetry instruments, registered against the global OTel
// MeterProvider (wired in internal/platform/telemetry.go via the OTLP/gRPC
// metric exporter -> the ACA managed-environment OTel agent -> App Insights
// AppMetrics). It covers the PlanIt error rate, notification, push and
// watch-zone counters the SRE team monitors.
//
// Instrument names and tag keys are stable: changing them would
// break the App Insights dashboards and alerts that key on them.
//
// Registry exposes recording METHODS rather than raw instruments so every call
// site stays trivial and the tag conventions live in one place. Recording
// packages depend on a small consumer-side interface (the methods they use), not
// on this concrete type, so *Registry satisfies them structurally. A nil
// *Registry is a no-op on every method, so a call site can hold a nil registry
// when telemetry is unconfigured without branching.
package metrics

import (
	"context"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// MeterName is the instrumentation scope for the business metrics. The OTel
// agent maps it to the App Insights AppMetrics SDK version tag; the towncrier.*
// instrument names are what dashboards key on, so the scope name itself is not
// load-bearing.
const MeterName = "towncrier"

// Registry holds the registered towncrier.* instruments. Build it once at boot
// from otel.Meter(MeterName) (after platform.SetupTelemetry has installed the
// global MeterProvider) and inject it through constructors. All instrument
// construction errors are swallowed at build time — a failed instrument records
// nothing rather than failing the boot, matching the OTel "metrics never break
// the app" contract.
type Registry struct {
	planitHTTPErrors metric.Int64Counter

	notificationsCreated metric.Int64Counter

	watchZonesCreated metric.Int64Counter
	watchZonesUpdated metric.Int64Counter
	watchZonesDeleted metric.Int64Counter

	pushDeliveryFailed metric.Int64Counter
}

// New builds the registry from a meter. Construction errors are deliberately
// dropped: a missing instrument no-ops on record rather than aborting startup.
func New(meter metric.Meter) *Registry {
	r := &Registry{}

	// counter / histogram / gauge drop the construction error deliberately: a
	// failed instrument is left nil, and every recording method nil-checks it and
	// no-ops, so a metric never aborts startup or a request. This is the OTel
	// "metrics must not break the app" contract, and it keeps each registration
	// below to one readable line while satisfying errcheck.
	counter := func(name string, opts ...metric.Int64CounterOption) metric.Int64Counter {
		inst, err := meter.Int64Counter(name, opts...)
		if err != nil {
			return nil
		}
		return inst
	}

	r.planitHTTPErrors = counter(
		"towncrier.planit.http_errors",
		metric.WithDescription("Non-2xx HTTP responses from PlanIt API"),
	)

	r.notificationsCreated = counter(
		"towncrier.notifications.created",
		metric.WithDescription("Notification records created (may or may not result in push)"),
	)

	r.watchZonesCreated = counter("towncrier.watchzones.created")
	r.watchZonesUpdated = counter("towncrier.watchzones.updated")
	r.watchZonesDeleted = counter("towncrier.watchzones.deleted")

	r.pushDeliveryFailed = counter(
		"towncrier.push.delivery_failed",
		metric.WithDescription("Per-device push send delivery failures (APNs or FCM), tagged by platform. Excludes routine invalid-token rejections (410/BadDeviceToken/UNREGISTERED etc.), which are expected churn the caller prunes, not a failure."),
	)

	return r
}

// PlanItHTTPError counts a non-2xx PlanIt response, tagged with the status code
// and authority. The tag keys (http.response.status_code, planit.authority_code)
// are stable so existing App Insights queries keep working.
func (r *Registry) PlanItHTTPError(ctx context.Context, statusCode, authorityID int) {
	if r == nil || r.planitHTTPErrors == nil {
		return
	}
	r.planitHTTPErrors.Add(ctx, 1, metric.WithAttributes(
		attribute.Int("http.response.status_code", statusCode),
		attribute.Int("planit.authority_code", authorityID),
	))
}

// NotificationCreated counts a notification record created, tagged by event_type
// ("NewApplication" | "DecisionUpdate") and sources ("Zone" | "Saved" | both).
func (r *Registry) NotificationCreated(ctx context.Context, eventType, sources string) {
	if r == nil || r.notificationsCreated == nil {
		return
	}
	r.notificationsCreated.Add(ctx, 1, metric.WithAttributes(
		attribute.String("event_type", eventType),
		attribute.String("sources", sources),
	))
}

// WatchZoneCreated counts a watch zone created.
func (r *Registry) WatchZoneCreated(ctx context.Context) {
	if r == nil || r.watchZonesCreated == nil {
		return
	}
	r.watchZonesCreated.Add(ctx, 1)
}

// WatchZoneUpdated counts a watch zone updated.
func (r *Registry) WatchZoneUpdated(ctx context.Context) {
	if r == nil || r.watchZonesUpdated == nil {
		return
	}
	r.watchZonesUpdated.Add(ctx, 1)
}

// WatchZoneDeleted counts a watch zone deleted.
func (r *Registry) WatchZoneDeleted(ctx context.Context) {
	if r == nil || r.watchZonesDeleted == nil {
		return
	}
	r.watchZonesDeleted.Add(ctx, 1)
}

// PushDeliveryFailed counts a per-device push send delivery failure (JWT/token
// mint failure, transport error, or exhausted retries) for the given platform
// ("apns" | "fcm"). Deliberately excludes a routine invalid-token rejection
// (410 Unregistered, 400 BadDeviceToken, UNREGISTERED, ...) — that is expected
// token churn the caller prunes, not a failure worth alerting on.
func (r *Registry) PushDeliveryFailed(ctx context.Context, platform string) {
	if r == nil || r.pushDeliveryFailed == nil {
		return
	}
	r.pushDeliveryFailed.Add(ctx, 1, metric.WithAttributes(attribute.String("platform", platform)))
}

// boolStr renders a bool as the lowercase "true"/"false" string used in App
// Insights metric tags.
func boolStr(b bool) string {
	if b {
		return "true"
	}
	return "false"
}
