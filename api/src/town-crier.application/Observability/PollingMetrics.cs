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

    public static readonly Counter<long> CursorAdvanced =
        Meter.CreateCounter<long>(
            "towncrier.polling.cursor_advanced",
            description: "Incremented when the handler persists a non-null cursor (cap hit or 429 mid-pagination). Tagged by cycle.type.");

    public static readonly Counter<long> CursorCleared =
        Meter.CreateCounter<long>(
            "towncrier.polling.cursor_cleared",
            description: "Incremented when the handler clears a previously-active cursor after reaching a natural end. Tagged by cycle.type.");
}
#pragma warning restore SA1202
