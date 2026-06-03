using TownCrier.Application.Notifications;
using TownCrier.Domain.Notifications;

namespace TownCrier.Application.Tests.Notifications;

internal sealed class FakeNotificationRepository : INotificationRepository
{
    private readonly List<Notification> store = [];

    public IReadOnlyList<Notification> All => this.store;

    /// <summary>
    /// Gets number of times the batched latest-unread lookup has been invoked. The
    /// applications-by-zone handler must call it exactly once per request regardless
    /// of how many applications are in scope (bd tc-1wkp).
    /// </summary>
    public int GetLatestUnreadByApplicationsCallCount { get; private set; }

    public void Seed(Notification notification)
    {
        this.store.Add(notification);
    }

    public Task<Notification?> GetByUserAndApplicationAsync(
        string userId,
        string applicationUid,
        int authorityId,
        NotificationEventType eventType,
        CancellationToken ct)
    {
        var notification = this.store.Find(
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
        // Watermark semantics: a notification is unread iff its CreatedAt is
        // strictly greater than lastReadAt. The boundary instant itself is read.
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
        this.GetLatestUnreadByApplicationsCallCount++;

        var uids = new HashSet<string>(applicationUids, StringComparer.Ordinal);

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
        this.store.RemoveAll(n => n.UserId == userId);
        return Task.CompletedTask;
    }
}
