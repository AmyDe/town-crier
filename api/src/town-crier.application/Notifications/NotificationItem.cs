namespace TownCrier.Application.Notifications;

public sealed record NotificationItem(
    string ApplicationName,
    string ApplicationAddress,
    string ApplicationDescription,
    string? ApplicationType,
    int AuthorityId,
    DateTimeOffset CreatedAt,
    string EventType,
    string? Decision,
    string Sources);
