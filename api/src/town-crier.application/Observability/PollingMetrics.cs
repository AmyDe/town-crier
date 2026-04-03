using System.Diagnostics.Metrics;

namespace TownCrier.Application.Observability;

public static class PollingMetrics
{
    public const string MeterName = "TownCrier.Polling";

    public static readonly Meter PollingMeter = new(MeterName);

    public static readonly Counter<long> AuthoritiesPolled =
        PollingMeter.CreateCounter<long>("towncrier.polling.authorities_polled");

    public static readonly Counter<long> AuthoritiesSkipped =
        PollingMeter.CreateCounter<long>("towncrier.polling.authorities_skipped");

    public static readonly Counter<long> ApplicationsIngested =
        PollingMeter.CreateCounter<long>("towncrier.polling.applications_ingested");

    public static readonly Counter<long> PollFailures =
        PollingMeter.CreateCounter<long>("towncrier.polling.failures");

    public static readonly Histogram<double> PlanItLatency =
        PollingMeter.CreateHistogram<double>(
            "towncrier.polling.planit_latency_ms",
            unit: "ms",
            description: "PlanIt API response time");

    public static readonly Histogram<double> CycleDuration =
        PollingMeter.CreateHistogram<double>(
            "towncrier.polling.cycle_duration_ms",
            unit: "ms",
            description: "Full polling cycle duration");
}
