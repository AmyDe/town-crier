using System.Diagnostics;
using System.Diagnostics.Metrics;

namespace TownCrier.Infrastructure.Observability;

#pragma warning disable SA1202 // Meter must be initialized before public fields that reference it
public static class CosmosInstrumentation
{
    public const string ActivitySourceName = "TownCrier.Cosmos";
    public const string MeterName = "TownCrier.Cosmos";

    public static readonly ActivitySource Source = new(ActivitySourceName);
    private static readonly Meter Meter = new(MeterName);

    public static readonly Histogram<double> RequestCharge =
        Meter.CreateHistogram<double>(
            "towncrier.cosmos.request_charge_ru",
            unit: "RU",
            description: "Cosmos RU consumption per operation");

    public static readonly Counter<long> Throttles =
        Meter.CreateCounter<long>(
            "towncrier.cosmos.throttles",
            description: "429 responses from Cosmos");
}
#pragma warning restore SA1202
