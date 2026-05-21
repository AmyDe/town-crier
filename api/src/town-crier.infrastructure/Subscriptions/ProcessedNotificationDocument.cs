namespace TownCrier.Infrastructure.Subscriptions;

/// <summary>
/// Cosmos document recording that an App Store Server Notification has been
/// processed. The document <c>id</c> and partition key are both the Apple
/// <c>notificationUUID</c>, so a duplicate delivery is detected with a single
/// point read.
/// </summary>
internal sealed class ProcessedNotificationDocument
{
    public required string Id { get; init; }

    public required DateTimeOffset ProcessedAt { get; init; }
}
