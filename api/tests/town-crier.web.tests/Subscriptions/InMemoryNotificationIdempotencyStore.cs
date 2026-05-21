using System.Collections.Concurrent;
using TownCrier.Application.Subscriptions;

namespace TownCrier.Web.Tests.Subscriptions;

/// <summary>
/// In-memory <see cref="INotificationIdempotencyStore"/> for endpoint tests —
/// substitutes for the Cosmos-backed store so the webhook can be exercised
/// without a database.
/// </summary>
internal sealed class InMemoryNotificationIdempotencyStore : INotificationIdempotencyStore
{
    private readonly ConcurrentDictionary<string, byte> processed = new();

    public Task<bool> IsProcessedAsync(string notificationUuid, CancellationToken ct) =>
        Task.FromResult(this.processed.ContainsKey(notificationUuid));

    public Task MarkProcessedAsync(string notificationUuid, CancellationToken ct)
    {
        this.processed[notificationUuid] = 0;
        return Task.CompletedTask;
    }
}
