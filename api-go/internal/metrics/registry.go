// Package metrics holds the Town Crier business-metric registry: the
// towncrier.* OpenTelemetry instruments the .NET worker/api emitted before the
// Go cutover, re-registered against the global OTel MeterProvider (wired in
// internal/platform/telemetry.go via the OTLP/gRPC metric exporter -> the ACA
// managed-environment OTel agent -> App Insights AppMetrics). It restores the
// polling-pipeline KPIs, PlanIt error rate, Cosmos RU consumption and watch-zone
// counters the SRE team monitors on (bead tc-21np).
//
// The instrument NAMES and the tag keys (cycle.type, polling.authority_code,
// never_polled, header_present, caller, ...) match the .NET implementation
// exactly so the existing App Insights dashboards and alerts that key on them
// keep working across the cutover.
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
	authoritiesPolled  metric.Int64Counter
	authoritiesSkipped metric.Int64Counter
	applicationsIngest metric.Int64Counter
	rateLimited        metric.Int64Counter
	retryAfterSeconds  metric.Float64Histogram
	authorityProcMs    metric.Float64Histogram
	authorityTotal     metric.Int64Gauge
	cyclesCompleted    metric.Int64Counter
	cursorAdvanced     metric.Int64Counter
	cursorCleared      metric.Int64Counter
	leaseAcquired      metric.Int64Counter
	oldestHwmAge       metric.Float64Gauge
	neverPolledCount   metric.Int64Gauge

	planitHTTPErrors metric.Int64Counter

	notificationsCreated metric.Int64Counter

	cosmosRequestCharge metric.Float64Histogram

	watchZonesCreated metric.Int64Counter
	watchZonesUpdated metric.Int64Counter
	watchZonesDeleted metric.Int64Counter
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
	histogram := func(name string, opts ...metric.Float64HistogramOption) metric.Float64Histogram {
		inst, err := meter.Float64Histogram(name, opts...)
		if err != nil {
			return nil
		}
		return inst
	}
	intGauge := func(name string, opts ...metric.Int64GaugeOption) metric.Int64Gauge {
		inst, err := meter.Int64Gauge(name, opts...)
		if err != nil {
			return nil
		}
		return inst
	}
	floatGauge := func(name string, opts ...metric.Float64GaugeOption) metric.Float64Gauge {
		inst, err := meter.Float64Gauge(name, opts...)
		if err != nil {
			return nil
		}
		return inst
	}

	r.authoritiesPolled = counter("towncrier.polling.authorities_polled")
	r.authoritiesSkipped = counter("towncrier.polling.authorities_skipped")
	r.applicationsIngest = counter("towncrier.polling.applications_ingested")
	r.rateLimited = counter("towncrier.polling.rate_limited")
	r.retryAfterSeconds = histogram(
		"towncrier.polling.retry_after_seconds",
		metric.WithUnit("s"),
		metric.WithDescription("PlanIt-supplied Retry-After value (seconds) on 429 responses. Tagged by cycle.type, polling.authority_code, and header_present."),
	)
	r.authorityProcMs = histogram(
		"towncrier.polling.authority_processing_ms",
		metric.WithUnit("ms"),
		metric.WithDescription("Total per-authority processing time (fetch + upsert + notifications)"),
	)
	r.authorityTotal = intGauge(
		"towncrier.polling.authority_total",
		metric.WithDescription("PlanIt-reported total matching applications for an authority at the start of a page-1 fetch. Tagged by cycle.type and polling.authority_code."),
	)
	r.cyclesCompleted = counter(
		"towncrier.polling.cycles_completed",
		metric.WithDescription("Finished poll cycles, tagged by cycle.type and termination."),
	)
	r.cursorAdvanced = counter(
		"towncrier.polling.cursor_advanced",
		metric.WithDescription("Incremented when the handler persists a non-null cursor (cap hit or 429 mid-pagination). Tagged by cycle.type."),
	)
	r.cursorCleared = counter(
		"towncrier.polling.cursor_cleared",
		metric.WithDescription("Incremented when the handler clears a previously-active cursor after reaching a natural end. Tagged by cycle.type."),
	)
	r.leaseAcquired = counter(
		"towncrier.polling.lease.acquired",
		metric.WithDescription("Incremented when the polling lease is successfully acquired. Tagged by caller."),
	)
	r.oldestHwmAge = floatGauge(
		"towncrier.polling.oldest_hwm_age_seconds",
		metric.WithUnit("s"),
		metric.WithDescription("Age, in seconds, of the stalest authority's LastPollTime at the start of a cycle. Tagged by cycle.type, polling.authority_code, and never_polled."),
	)
	r.neverPolledCount = intGauge(
		"towncrier.polling.never_polled_count",
		metric.WithDescription("Count of authorities with no PollState document at the start of a cycle. Tagged by cycle.type."),
	)

	r.planitHTTPErrors = counter(
		"towncrier.planit.http_errors",
		metric.WithDescription("Non-2xx HTTP responses from PlanIt API"),
	)

	r.notificationsCreated = counter(
		"towncrier.notifications.created",
		metric.WithDescription("Notification records created (may or may not result in push)"),
	)

	r.cosmosRequestCharge = histogram(
		"towncrier.cosmos.request_charge_ru",
		metric.WithUnit("RU"),
		metric.WithDescription("Cosmos RU consumption per operation"),
	)

	r.watchZonesCreated = counter("towncrier.watchzones.created")
	r.watchZonesUpdated = counter("towncrier.watchzones.updated")
	r.watchZonesDeleted = counter("towncrier.watchzones.deleted")

	return r
}

// cycleTag builds the cycle.type attribute every polling instrument carries.
func cycleTag(cycleType string) attribute.KeyValue {
	return attribute.String("cycle.type", cycleType)
}

// AuthorityPolled counts an authority that produced work this cycle.
func (r *Registry) AuthorityPolled(ctx context.Context, cycleType string) {
	if r == nil || r.authoritiesPolled == nil {
		return
	}
	r.authoritiesPolled.Add(ctx, 1, metric.WithAttributes(cycleTag(cycleType)))
}

// AuthoritySkipped counts an authority skipped this cycle (no work / error).
func (r *Registry) AuthoritySkipped(ctx context.Context, cycleType string) {
	if r == nil || r.authoritiesSkipped == nil {
		return
	}
	r.authoritiesSkipped.Add(ctx, 1, metric.WithAttributes(cycleTag(cycleType)))
}

// ApplicationsIngested adds the count of applications ingested for an authority.
func (r *Registry) ApplicationsIngested(ctx context.Context, n int, cycleType string) {
	if r == nil || r.applicationsIngest == nil {
		return
	}
	r.applicationsIngest.Add(ctx, int64(n), metric.WithAttributes(cycleTag(cycleType)))
}

// RateLimited counts a 429 from PlanIt during this cycle.
func (r *Registry) RateLimited(ctx context.Context, cycleType string) {
	if r == nil || r.rateLimited == nil {
		return
	}
	r.rateLimited.Add(ctx, 1, metric.WithAttributes(cycleTag(cycleType)))
}

// RetryAfterSeconds records the parsed Retry-After value (seconds) on a 429,
// tagged with cycle.type, polling.authority_code and header_present so
// dashboards can distinguish "no header" (headerPresent=false, value 0) from a
// genuine small backoff.
func (r *Registry) RetryAfterSeconds(ctx context.Context, seconds float64, cycleType string, authorityID int, headerPresent bool) {
	if r == nil || r.retryAfterSeconds == nil {
		return
	}
	r.retryAfterSeconds.Record(ctx, seconds, metric.WithAttributes(
		cycleTag(cycleType),
		attribute.Int("polling.authority_code", authorityID),
		attribute.String("header_present", boolStr(headerPresent)),
	))
}

// AuthorityProcessingMillis records the total per-authority processing time.
func (r *Registry) AuthorityProcessingMillis(ctx context.Context, ms float64, cycleType string) {
	if r == nil || r.authorityProcMs == nil {
		return
	}
	r.authorityProcMs.Record(ctx, ms, metric.WithAttributes(cycleTag(cycleType)))
}

// AuthorityTotal records PlanIt's reported total matching applications for an
// authority at the start of a page-1 fetch.
func (r *Registry) AuthorityTotal(ctx context.Context, total int, cycleType string, authorityID int) {
	if r == nil || r.authorityTotal == nil {
		return
	}
	r.authorityTotal.Record(ctx, int64(total), metric.WithAttributes(
		cycleTag(cycleType),
		attribute.Int("polling.authority_code", authorityID),
	))
}

// CycleCompleted counts a finished poll cycle, tagged by cycle.type and the
// termination reason.
func (r *Registry) CycleCompleted(ctx context.Context, cycleType, termination string) {
	if r == nil || r.cyclesCompleted == nil {
		return
	}
	r.cyclesCompleted.Add(ctx, 1, metric.WithAttributes(
		cycleTag(cycleType),
		attribute.String("termination", termination),
	))
}

// CursorAdvanced counts a non-null cursor persist (cap hit / 429 mid-pagination).
func (r *Registry) CursorAdvanced(ctx context.Context, cycleType string) {
	if r == nil || r.cursorAdvanced == nil {
		return
	}
	r.cursorAdvanced.Add(ctx, 1, metric.WithAttributes(cycleTag(cycleType)))
}

// CursorCleared counts clearing a previously-active cursor at a natural end.
func (r *Registry) CursorCleared(ctx context.Context, cycleType string) {
	if r == nil || r.cursorCleared == nil {
		return
	}
	r.cursorCleared.Add(ctx, 1, metric.WithAttributes(cycleTag(cycleType)))
}

// LeaseAcquired counts a successful polling-lease acquisition, tagged by caller
// ("orchestrator" | "bootstrap").
func (r *Registry) LeaseAcquired(ctx context.Context, caller string) {
	if r == nil || r.leaseAcquired == nil {
		return
	}
	r.leaseAcquired.Add(ctx, 1, metric.WithAttributes(attribute.String("caller", caller)))
}

// OldestHighWaterMarkAge records the age (seconds) of the stalest authority's
// LastPollTime at cycle start, tagged with cycle.type, polling.authority_code
// and never_polled.
func (r *Registry) OldestHighWaterMarkAge(ctx context.Context, seconds float64, cycleType string, authorityID int, neverPolled bool) {
	if r == nil || r.oldestHwmAge == nil {
		return
	}
	r.oldestHwmAge.Record(ctx, seconds, metric.WithAttributes(
		cycleTag(cycleType),
		attribute.Int("polling.authority_code", authorityID),
		attribute.String("never_polled", boolStr(neverPolled)),
	))
}

// NeverPolledCount records the number of candidate authorities with no PollState
// document at cycle start.
func (r *Registry) NeverPolledCount(ctx context.Context, count int, cycleType string) {
	if r == nil || r.neverPolledCount == nil {
		return
	}
	r.neverPolledCount.Record(ctx, int64(count), metric.WithAttributes(cycleTag(cycleType)))
}

// PlanItHTTPError counts a non-2xx PlanIt response, tagged with the status code
// and authority. The tag keys (http.response.status_code, planit.authority_code)
// match the .NET PlanItClient instrumentation so existing App Insights queries
// keep working.
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
// ("NewApplication" | "DecisionUpdate") and sources ("Zone" | "Saved" | both),
// matching the .NET ApiMetrics.NotificationsCreated tags.
func (r *Registry) NotificationCreated(ctx context.Context, eventType, sources string) {
	if r == nil || r.notificationsCreated == nil {
		return
	}
	r.notificationsCreated.Add(ctx, 1, metric.WithAttributes(
		attribute.String("event_type", eventType),
		attribute.String("sources", sources),
	))
}

// CosmosRequestCharge records the RU charge of a single Cosmos operation, tagged
// with the operation and container.
func (r *Registry) CosmosRequestCharge(ctx context.Context, ru float64, operation, container string) {
	if r == nil || r.cosmosRequestCharge == nil {
		return
	}
	r.cosmosRequestCharge.Record(ctx, ru, metric.WithAttributes(
		attribute.String("operation", operation),
		attribute.String("container", container),
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

// boolStr renders a bool as the lowercase "true"/"false" string the .NET tags
// used, so App Insights filters keep matching across the cutover.
func boolStr(b bool) string {
	if b {
		return "true"
	}
	return "false"
}
