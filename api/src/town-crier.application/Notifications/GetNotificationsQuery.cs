namespace TownCrier.Application.Notifications;

public sealed record GetNotificationsQuery(
    string UserId,
    int Page = 1,
    int PageSize = 20);
