using System.Diagnostics.Metrics;

namespace TownCrier.Application.Observability;

#pragma warning disable SA1202 // Meter must be initialized before public fields that reference it
public static class ApiMetrics
{
    public const string MeterName = "TownCrier.Api";

    private static readonly Meter Meter = new(MeterName);

    public static readonly Counter<long> WatchZonesCreated =
        Meter.CreateCounter<long>("towncrier.watchzones.created");

    public static readonly Counter<long> NotificationsSent =
        Meter.CreateCounter<long>("towncrier.notifications.sent");

    public static readonly UpDownCounter<long> ActiveSubscriptions =
        Meter.CreateUpDownCounter<long>("towncrier.subscriptions.active");

    public static readonly Counter<long> EndpointErrors =
        Meter.CreateCounter<long>(
            "towncrier.api.errors",
            description: "Unhandled exceptions by endpoint");
}
#pragma warning restore SA1202
