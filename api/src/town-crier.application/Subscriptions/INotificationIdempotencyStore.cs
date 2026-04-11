namespace TownCrier.Application.Subscriptions;

/// <summary>
/// Stores processed App Store notification UUIDs for idempotency.
/// </summary>
public interface INotificationIdempotencyStore
{
    Task<bool> IsProcessedAsync(string notificationUuid, CancellationToken ct);

    Task MarkProcessedAsync(string notificationUuid, CancellationToken ct);
}
