using TownCrier.Domain.Notifications;

namespace TownCrier.Application.Notifications;

public interface INotificationRepository
{
    Task<Notification?> GetByUserAndApplicationAsync(
        string userId,
        string applicationUid,
        NotificationEventType eventType,
        CancellationToken ct);

    Task<int> CountByUserSinceAsync(string userId, DateTimeOffset since, CancellationToken ct);

    Task<IReadOnlyList<Notification>> GetByUserSinceAsync(string userId, DateTimeOffset since, CancellationToken ct);

    Task<(IReadOnlyList<Notification> Items, int Total)> GetByUserPaginatedAsync(
        string userId, int page, int pageSize, CancellationToken ct);

    Task<IReadOnlyList<Notification>> GetUnsentEmailsByUserAsync(string userId, CancellationToken ct);

    Task<IReadOnlyList<string>> GetUserIdsWithUnsentEmailsAsync(CancellationToken ct);

    Task SaveAsync(Notification notification, CancellationToken ct);

    Task DeleteAllByUserIdAsync(string userId, CancellationToken ct);
}
