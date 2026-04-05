using System.Diagnostics.Metrics;

namespace TownCrier.Infrastructure.Observability;

#pragma warning disable SA1202 // Meter must be initialized before public fields that reference it
public static class PlanItInstrumentation
{
    public const string MeterName = "TownCrier.PlanIt";

    private static readonly Meter Meter = new(MeterName);

    public static readonly Counter<long> HttpErrors =
        Meter.CreateCounter<long>(
            "towncrier.planit.http_errors",
            description: "Non-2xx HTTP responses from PlanIt API");
}
#pragma warning restore SA1202
