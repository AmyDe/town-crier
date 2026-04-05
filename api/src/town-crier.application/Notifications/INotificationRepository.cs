using TownCrier.Domain.Notifications;

namespace TownCrier.Application.Notifications;

public interface INotificationRepository
{
    Task<Notification?> GetByUserAndApplicationAsync(string userId, string applicationName, CancellationToken ct);

    Task<int> CountByUserInMonthAsync(string userId, int year, int month, CancellationToken ct);

    Task<int> CountByUserSinceAsync(string userId, DateTimeOffset since, CancellationToken ct);

    Task<IReadOnlyList<Notification>> GetByUserSinceAsync(string userId, DateTimeOffset since, CancellationToken ct);

    Task<(IReadOnlyList<Notification> Items, int Total)> GetByUserPaginatedAsync(
        string userId, int page, int pageSize, CancellationToken ct);

    Task SaveAsync(Notification notification, CancellationToken ct);
}
