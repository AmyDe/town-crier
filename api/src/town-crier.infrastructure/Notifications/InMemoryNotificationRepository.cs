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
        int authorityId,
        NotificationEventType eventType,
        CancellationToken ct)
    {
        var notification = this.store.FirstOrDefault(
            n => n.UserId == userId
                && n.ApplicationUid == applicationUid
                && n.AuthorityId == authorityId
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

    public Task<IReadOnlyDictionary<string, Notification>> GetLatestUnreadByApplicationsAsync(
        string userId,
        IReadOnlyCollection<string> applicationUids,
        DateTimeOffset lastReadAt,
        CancellationToken ct)
    {
        var uids = applicationUids as ISet<string> ?? new HashSet<string>(applicationUids, StringComparer.Ordinal);

        var map = this.store
            .Where(n => n.UserId == userId
                && n.CreatedAt > lastReadAt
                && uids.Contains(n.ApplicationUid))
            .GroupBy(n => n.ApplicationUid, StringComparer.Ordinal)
            .ToDictionary(
                g => g.Key,
                g => g.OrderByDescending(n => n.CreatedAt).First(),
                StringComparer.Ordinal);

        return Task.FromResult<IReadOnlyDictionary<string, Notification>>(map);
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

    public Task<IReadOnlyList<string>> GetUserIdsWithUnsentEmailsCrossPartitionAsync(CancellationToken ct)
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
