using TownCrier.Application.Notifications;
using TownCrier.Domain.Notifications;

namespace TownCrier.Application.Tests.Notifications;

internal sealed class FakeNotificationRepository : INotificationRepository
{
    private readonly List<Notification> store = [];

    public IReadOnlyList<Notification> All => this.store;

    public Task<Notification?> GetByUserAndApplicationAsync(string userId, string applicationName, CancellationToken ct)
    {
        var notification = this.store.Find(
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

    public Task SaveAsync(Notification notification, CancellationToken ct)
    {
        this.store.Add(notification);
        return Task.CompletedTask;
    }
}
