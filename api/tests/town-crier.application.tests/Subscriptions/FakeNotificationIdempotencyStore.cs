using TownCrier.Application.Subscriptions;

namespace TownCrier.Application.Tests.Subscriptions;

internal sealed class FakeNotificationIdempotencyStore : INotificationIdempotencyStore
{
    private readonly HashSet<string> processed = [];

    public List<string> MarkedProcessed { get; } = [];

    public Task<bool> IsProcessedAsync(string notificationUuid, CancellationToken ct)
    {
        return Task.FromResult(this.processed.Contains(notificationUuid));
    }

    public Task MarkProcessedAsync(string notificationUuid, CancellationToken ct)
    {
        this.processed.Add(notificationUuid);
        this.MarkedProcessed.Add(notificationUuid);
        return Task.CompletedTask;
    }

    public void SeedProcessed(string notificationUuid)
    {
        this.processed.Add(notificationUuid);
    }
}
