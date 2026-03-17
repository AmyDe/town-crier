using System.Collections.Concurrent;
using TownCrier.Application.Notifications;
using TownCrier.Domain.Notifications;

namespace TownCrier.Infrastructure.Notifications;

public sealed class InMemoryNotificationRepository : INotificationRepository
{
    private readonly ConcurrentBag<Notification> store = [];

    public Task<Notification?> GetByUserAndApplicationAsync(string userId, string applicationName, CancellationToken ct)
    {
        var notification = this.store.FirstOrDefault(
            n => n.UserId == userId && n.ApplicationName == applicationName);
        return Task.FromResult(notification);
    }

    public Task<int> CountByUserInMonthAsync(string userId, int year, int month, CancellationToken ct)
    {
        var count = this.store.Count(n =>
            n.UserId == userId &&
            n.CreatedAt.Year == year &&
            n.CreatedAt.Month == month);
        return Task.FromResult(count);
    }

    public Task<int> CountByUserSinceAsync(string userId, DateTimeOffset since, CancellationToken ct)
    {
        var count = this.store.Count(n =>
            n.UserId == userId &&
            n.CreatedAt >= since);
        return Task.FromResult(count);
    }

    public Task<(IReadOnlyList<Notification> Items, int Total)> GetByUserPaginatedAsync(
        string userId, int page, int pageSize, CancellationToken ct)
    {
        var userNotifications = this.store
            .Where(n => n.UserId == userId)
            .OrderByDescending(n => n.CreatedAt)
            .ToList();

        var total = userNotifications.Count;
        var items = userNotifications
            .Skip((page - 1) * pageSize)
            .Take(pageSize)
            .ToList();

        return Task.FromResult<(IReadOnlyList<Notification> Items, int Total)>((items, total));
    }

    public Task SaveAsync(Notification notification, CancellationToken ct)
    {
        this.store.Add(notification);
        return Task.CompletedTask;
    }
}
