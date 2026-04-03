using System.Diagnostics;
using System.Diagnostics.Metrics;

namespace TownCrier.Infrastructure.Observability;

internal static class CosmosInstrumentation
{
    public const string ActivitySourceName = "TownCrier.Cosmos";
    public const string MeterName = "TownCrier.Cosmos";

    public static readonly ActivitySource Source = new(ActivitySourceName);
    public static readonly Meter CosmosMeter = new(MeterName);

    public static readonly Histogram<double> RequestCharge =
        CosmosMeter.CreateHistogram<double>(
            "towncrier.cosmos.request_charge_ru",
            unit: "RU",
            description: "Cosmos RU consumption per operation");

    public static readonly Counter<long> Throttles =
        CosmosMeter.CreateCounter<long>(
            "towncrier.cosmos.throttles",
            description: "429 responses from Cosmos");
}
