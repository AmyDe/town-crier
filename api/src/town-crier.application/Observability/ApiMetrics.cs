using System.Diagnostics.Metrics;

namespace TownCrier.Application.Observability;

public static class ApiMetrics
{
    public const string MeterName = "TownCrier.Api";

    public static readonly Meter ApiMeter = new(MeterName);

    public static readonly Counter<long> WatchZonesCreated =
        ApiMeter.CreateCounter<long>("towncrier.watchzones.created");

    public static readonly Counter<long> NotificationsSent =
        ApiMeter.CreateCounter<long>("towncrier.notifications.sent");

    public static readonly UpDownCounter<long> ActiveSubscriptions =
        ApiMeter.CreateUpDownCounter<long>("towncrier.subscriptions.active");

    public static readonly Counter<long> EndpointErrors =
        ApiMeter.CreateCounter<long>(
            "towncrier.api.errors",
            description: "Unhandled exceptions by endpoint");
}
