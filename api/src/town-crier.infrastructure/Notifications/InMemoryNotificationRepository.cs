using System.Collections.Concurrent;
using TownCrier.Application.Notifications;
using TownCrier.Domain.Notifications;

namespace TownCrier.Infrastructure.Notifications;

public sealed class InMemoryNotificationRepository : INotificationRepository
{
    private readonly ConcurrentBag<Notification> store = [];

    public Task<Notification?> GetByUserAndApplicationAsync(
        string userId,
        string applicationUid,
        NotificationEventType eventType,
        CancellationToken ct)
    {
        var notification = this.store.FirstOrDefault(
            n => n.UserId == userId
                && n.ApplicationUid == applicationUid
                && n.EventType == eventType);
        return Task.FromResult(notification);
    }

    public Task<int> CountByUserSinceAsync(string userId, DateTimeOffset since, CancellationToken ct)
    {
        var count = this.store.Count(n =>
            n.UserId == userId &&
            n.CreatedAt >= since);
        return Task.FromResult(count);
    }

    public Task<int> GetUnreadCountAsync(string userId, DateTimeOffset lastReadAt, CancellationToken ct)
    {
        // Watermark-aware unread count: strictly greater than lastReadAt. The
        // boundary instant itself counts as already read.
        var count = this.store.Count(n =>
            n.UserId == userId &&
            n.CreatedAt > lastReadAt);
        return Task.FromResult(count);
    }

    public Task<Notification?> GetLatestUnreadByApplicationAsync(
        string userId,
        string applicationUid,
        DateTimeOffset lastReadAt,
        CancellationToken ct)
    {
        // Strictly greater than the watermark, ordered by CreatedAt desc.
        var latest = this.store
            .Where(n => n.UserId == userId
                && n.ApplicationUid == applicationUid
                && n.CreatedAt > lastReadAt)
            .OrderByDescending(n => n.CreatedAt)
            .FirstOrDefault();
        return Task.FromResult(latest);
    }

    public Task<IReadOnlyList<Notification>> GetByUserSinceAsync(
        string userId, DateTimeOffset since, CancellationToken ct)
    {
        var notifications = this.store
            .Where(n => n.UserId == userId && n.CreatedAt >= since)
            .ToList();
        return Task.FromResult<IReadOnlyList<Notification>>(notifications);
    }

    public Task<IReadOnlyList<Notification>> GetUnsentEmailsByUserAsync(string userId, CancellationToken ct)
    {
        var notifications = this.store
            .Where(n => n.UserId == userId && !n.EmailSent)
            .OrderBy(n => n.CreatedAt)
            .ToList();
        return Task.FromResult<IReadOnlyList<Notification>>(notifications);
    }

    public Task<IReadOnlyList<string>> GetUserIdsWithUnsentEmailsAsync(CancellationToken ct)
    {
        var userIds = this.store
            .Where(n => !n.EmailSent)
            .Select(n => n.UserId)
            .Distinct()
            .ToList();
        return Task.FromResult<IReadOnlyList<string>>(userIds);
    }

    public Task SaveAsync(Notification notification, CancellationToken ct)
    {
        this.store.Add(notification);
        return Task.CompletedTask;
    }

    public Task DeleteAllByUserIdAsync(string userId, CancellationToken ct)
    {
        var remaining = this.store.Where(n => n.UserId != userId).ToList();
        while (this.store.TryTake(out _))
        {
            // Drain the bag.
        }

        foreach (var notification in remaining)
        {
            this.store.Add(notification);
        }

        return Task.CompletedTask;
    }
}
