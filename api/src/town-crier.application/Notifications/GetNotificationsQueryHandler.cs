namespace TownCrier.Application.Notifications;

public sealed class GetNotificationsQueryHandler
{
    private readonly INotificationRepository repository;

    public GetNotificationsQueryHandler(INotificationRepository repository)
    {
        this.repository = repository;
    }

    public async Task<GetNotificationsResult> HandleAsync(GetNotificationsQuery query, CancellationToken ct)
    {
        ArgumentNullException.ThrowIfNull(query);

        var (items, total) = await this.repository.GetByUserPaginatedAsync(
            query.UserId, query.Page, query.PageSize, ct).ConfigureAwait(false);

        var notifications = items
            .Select(n => new NotificationItem(
                n.ApplicationName,
                n.ApplicationAddress,
                n.ApplicationDescription,
                n.ApplicationType,
                n.AuthorityId,
                n.CreatedAt,
                n.EventType.ToString(),
                n.Decision,
                n.Sources.ToString()))
            .ToList();

        return new GetNotificationsResult(notifications, total, query.Page);
    }
}
