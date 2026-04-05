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

    public static readonly Counter<long> UsersRegistered =
        Meter.CreateCounter<long>("towncrier.users.registered");

    public static readonly Counter<long> WatchZonesDeleted =
        Meter.CreateCounter<long>("towncrier.watchzones.deleted");

    public static readonly Counter<long> SearchesPerformed =
        Meter.CreateCounter<long>("towncrier.search.performed");

    public static readonly Counter<long> NotificationsCreated =
        Meter.CreateCounter<long>(
            "towncrier.notifications.created",
            description: "Notification records created (may or may not result in push)");

    public static readonly Counter<long> EmailsSent =
        Meter.CreateCounter<long>(
            "towncrier.emails.sent",
            description: "Emails successfully queued for delivery");

    public static readonly Counter<long> EmailsFailed =
        Meter.CreateCounter<long>(
            "towncrier.emails.failed",
            description: "Email send failures");
}
#pragma warning restore SA1202
