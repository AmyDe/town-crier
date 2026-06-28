# 0034. iOS crash and stability reporting via MetricKit

Date: 2026-06-28

## Status

Accepted

See also [ADR 0018](0018-opentelemetry-observability-with-azure-monitor.md) and [ADR 0027](0027-go-api-observability-via-aca-otel-agent.md) (backend observability), which this complements on the client side.

## Context

The Go backend has full observability — OpenTelemetry traces, metrics, and logs flowing to Azure Monitor ([ADR 0018](0018-opentelemetry-observability-with-azure-monitor.md), [ADR 0027](0027-go-api-observability-via-aca-otel-agent.md)). The iOS app had no equivalent: a crash in the field left no signal at all, so field stability was invisible.

The obvious fix is a third-party crash SDK (Firebase Crashlytics, Sentry). Both are mature and give symbolicated, aggregated, near-real-time crash dashboards. But both also embed an SDK that collects device and installation identifiers and ships diagnostic payloads to a third-party backend. That conflicts directly with Town Crier's public privacy stance — "we do not track", no analytics pixels, no client-IP logging, data minimisation throughout (see the `no_track_privacy_stance` and `ip_logging_gdpr_minimisation` project context). Adding a tracking SDK purely for crash visibility would undercut a positioning the product makes explicitly to users, and would add a third-party dependency and binary weight for a pre-revenue solo app.

## Decision

**Use Apple's MetricKit for crash and stability reporting — first-party, on-device, no third-party SDK, no PII.** A `MetricKitCrashReporter` conforms to `MXMetricManagerSubscriber` and subscribes to MetricKit's diagnostic payloads (crash signal, termination reason, plus the stability and performance metrics MetricKit already gathers). Received payloads are written to the OS unified logger, where they can be read via Console.app or pulled from a device sysdiagnose. The OS batches and delivers these payloads on its own schedule (roughly 0–24 hours after the event); that latency is accepted as the cost of a zero-PII, zero-dependency approach.

## Consequences

### Easier

- **Crash visibility with no privacy regression.** MetricKit is Apple's own framework, runs entirely on-device, and carries no user or installation identifier, so it is consistent with the "we do not track" stance rather than in tension with it.
- **No third-party dependency.** No extra SDK to integrate, update, or audit, and no binary-size or supply-chain cost — it is part of the platform.
- **Aligned with the existing posture.** Stability data surfaces through the OS logging tools the app already uses, keeping the diagnostic surface inside Apple's sandbox.

### Harder

- **No real-time alerting.** Delivery is OS-batched (0–24 h), so MetricKit cannot page on a crash spike the way Crashlytics/Sentry can — it is a trend-and-postmortem signal, not an incident trigger.
- **No managed aggregation or symbolication dashboard.** There is no hosted UI grouping crashes by signature or auto-symbolicating stack traces; payloads are logged locally and must be collected (Console/sysdiagnose) or forwarded by future work if aggregation is ever needed.
- **Coarse at low volume.** With few devices in the field, the batched, sampled nature of MetricKit makes it weak for reproducing a specific one-off crash quickly. If real-time field debugging becomes necessary, this decision should be revisited against a privacy-preserving forwarding option.
