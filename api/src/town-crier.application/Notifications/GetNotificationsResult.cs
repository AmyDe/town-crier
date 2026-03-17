namespace TownCrier.Application.Notifications;

public sealed record GetNotificationsResult(
    IReadOnlyList<NotificationItem> Notifications,
    int Total,
    int Page);
