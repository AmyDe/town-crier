using System.Diagnostics.Metrics;

namespace TownCrier.Application.Observability;

#pragma warning disable SA1202 // Meter must be initialized before public fields that reference it
public static class PollingMetrics
{
    public const string MeterName = "TownCrier.Polling";

    private static readonly Meter Meter = new(MeterName);

    public static readonly Counter<long> AuthoritiesPolled =
        Meter.CreateCounter<long>("towncrier.polling.authorities_polled");

    public static readonly Counter<long> AuthoritiesSkipped =
        Meter.CreateCounter<long>("towncrier.polling.authorities_skipped");

    public static readonly Counter<long> ApplicationsIngested =
        Meter.CreateCounter<long>("towncrier.polling.applications_ingested");

    public static readonly Counter<long> PollFailures =
        Meter.CreateCounter<long>("towncrier.polling.failures");

    public static readonly Histogram<double> AuthorityProcessingDuration =
        Meter.CreateHistogram<double>(
            "towncrier.polling.authority_processing_ms",
            unit: "ms",
            description: "Total per-authority processing time (fetch + upsert + notifications)");

    public static readonly Counter<long> RateLimited =
        Meter.CreateCounter<long>("towncrier.polling.rate_limited");

    /// <summary>
    /// Distribution of parsed <c>Retry-After</c> values (in seconds) returned by PlanIt
    /// when a poll is throttled with HTTP 429. Recorded once per rate-limited authority,
    /// tagged with <c>cycle.type</c>, <c>polling.authority_code</c>, and
    /// <c>header_present</c> = "true" | "false". A value of 0 with header_present=false
    /// is emitted when PlanIt returned 429 without a Retry-After header so dashboards
    /// can distinguish "no header" from "small backoff". See bd tc-6nkn.
    /// </summary>
    public static readonly Histogram<double> RetryAfterSeconds =
        Meter.CreateHistogram<double>(
            "towncrier.polling.retry_after_seconds",
            unit: "s",
            description: "PlanIt-supplied Retry-After value (seconds) on 429 responses. Tagged by cycle.type, polling.authority_code, and header_present.");

    public static readonly Histogram<double> CycleDuration =
        Meter.CreateHistogram<double>(
            "towncrier.polling.cycle_duration_ms",
            unit: "ms",
            description: "Full polling cycle duration");

    public static readonly Counter<long> CyclesCompleted =
        Meter.CreateCounter<long>(
            "towncrier.polling.cycles_completed",
            description: "Finished poll cycles, tagged by cycle.type and termination.");

    public static readonly Gauge<long> AuthorityTotal =
        Meter.CreateGauge<long>(
            "towncrier.polling.authority_total",
            description: "PlanIt-reported total matching applications for an authority at the start of a page-1 fetch. Tagged by cycle.type and polling.authority_code. See docs/specs/polling-resumable-cursor.md#telemetry-additions.");

    public static readonly Gauge<double> OldestHighWaterMarkAge =
        Meter.CreateGauge<double>(
            "towncrier.polling.oldest_hwm_age_seconds",
            unit: "s",
            description: "Age, in seconds, of the stalest authority's LastPollTime at the start of a cycle. Tagged by cycle.type, polling.authority_code, and never_polled. A never-polled authority reports (now - UnixEpoch) and never_polled=true so dashboards can distinguish it from a genuinely stale HWM.");

    public static readonly Counter<long> CursorAdvanced =
        Meter.CreateCounter<long>(
            "towncrier.polling.cursor_advanced",
            description: "Incremented when the handler persists a non-null cursor (cap hit or 429 mid-pagination). Tagged by cycle.type.");

    public static readonly Counter<long> CursorCleared =
        Meter.CreateCounter<long>(
            "towncrier.polling.cursor_cleared",
            description: "Incremented when the handler clears a previously-active cursor after reaching a natural end. Tagged by cycle.type.");

    /// <summary>
    /// Incremented each time the caller successfully acquires the polling lease.
    /// Tagged with <c>caller</c> = "orchestrator" | "bootstrap".
    /// </summary>
    public static readonly Counter<long> LeaseAcquired =
        Meter.CreateCounter<long>(
            "towncrier.polling.lease.acquired",
            description: "Incremented when the polling lease is successfully acquired.");

    /// <summary>
    /// Incremented when the lease is unavailable after all retry attempts.
    /// Tagged with <c>caller</c> = "orchestrator" | "bootstrap".
    /// </summary>
    public static readonly Counter<long> LeaseHeldByPeer =
        Meter.CreateCounter<long>(
            "towncrier.polling.lease.held_by_peer",
            description: "Incremented when the polling lease is held by a peer and unavailable after retry.");

    /// <summary>
    /// Incremented when the conditional delete on release returns 412 Precondition Failed.
    /// Tagged with <c>caller</c> = "orchestrator" | "bootstrap".
    /// </summary>
    public static readonly Counter<long> LeaseReleased412 =
        Meter.CreateCounter<long>(
            "towncrier.polling.lease.released_412",
            description: "Incremented when the lease release fails with a precondition-failed (412) outcome.");
}
#pragma warning restore SA1202
