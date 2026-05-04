using TownCrier.Application.Notifications;
using TownCrier.Domain.Notifications;

namespace TownCrier.Application.Tests.Notifications;

internal sealed class FakeNotificationRepository : INotificationRepository
{
    private readonly List<Notification> store = [];

    public IReadOnlyList<Notification> All => this.store;

    public void Seed(Notification notification)
    {
        this.store.Add(notification);
    }

    public Task<Notification?> GetByUserAndApplicationAsync(
        string userId,
        string applicationUid,
        NotificationEventType eventType,
        CancellationToken ct)
    {
        var notification = this.store.Find(
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
        // Watermark semantics: a notification is unread iff its CreatedAt is
        // strictly greater than lastReadAt. The boundary instant itself is read.
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
        // Strictly greater than the watermark; pick the most-recent CreatedAt so
        // the row's status pill reflects the latest event for this application.
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
        this.store.RemoveAll(n => n.UserId == userId);
        return Task.CompletedTask;
    }
}
