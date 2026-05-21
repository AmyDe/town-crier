using TownCrier.Application.Subscriptions;
using TownCrier.Infrastructure.Cosmos;

namespace TownCrier.Infrastructure.Subscriptions;

/// <summary>
/// Cosmos-backed <see cref="INotificationIdempotencyStore"/>. Each processed
/// App Store Server Notification is recorded as one document on the
/// <c>AppleNotifications</c> container, keyed and partitioned by the Apple
/// <c>notificationUUID</c>. A duplicate delivery is therefore a no-op:
/// <see cref="IsProcessedAsync"/> finds the existing document and the handler
/// returns early.
/// </summary>
public sealed class CosmosNotificationIdempotencyStore : INotificationIdempotencyStore
{
    private readonly ICosmosRestClient client;
    private readonly TimeProvider timeProvider;

    public CosmosNotificationIdempotencyStore(ICosmosRestClient client)
        : this(client, TimeProvider.System)
    {
    }

    public CosmosNotificationIdempotencyStore(ICosmosRestClient client, TimeProvider timeProvider)
    {
        ArgumentNullException.ThrowIfNull(client);
        ArgumentNullException.ThrowIfNull(timeProvider);
        this.client = client;
        this.timeProvider = timeProvider;
    }

    public async Task<bool> IsProcessedAsync(string notificationUuid, CancellationToken ct)
    {
        ArgumentException.ThrowIfNullOrWhiteSpace(notificationUuid);

        var document = await this.client.ReadDocumentAsync(
            CosmosContainerNames.AppleNotifications,
            notificationUuid,
            notificationUuid,
            SubscriptionsCosmosJsonContext.Default.ProcessedNotificationDocument,
            ct).ConfigureAwait(false);

        return document is not null;
    }

    public async Task MarkProcessedAsync(string notificationUuid, CancellationToken ct)
    {
        ArgumentException.ThrowIfNullOrWhiteSpace(notificationUuid);

        var document = new ProcessedNotificationDocument
        {
            Id = notificationUuid,
            ProcessedAt = this.timeProvider.GetUtcNow(),
        };

        // Upsert (last-writer-wins) so a re-delivery that races past the
        // IsProcessedAsync check still completes without a conflict error.
        await this.client.UpsertDocumentAsync(
            CosmosContainerNames.AppleNotifications,
            document,
            notificationUuid,
            SubscriptionsCosmosJsonContext.Default.ProcessedNotificationDocument,
            ct).ConfigureAwait(false);
    }
}
