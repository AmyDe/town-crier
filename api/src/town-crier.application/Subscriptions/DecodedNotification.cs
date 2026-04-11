namespace TownCrier.Application.Subscriptions;

/// <summary>
/// Represents the decoded payload from an Apple App Store Server Notification v2.
/// </summary>
public sealed record DecodedNotification(
    string NotificationType,
    string? Subtype,
    string NotificationUuid,
    string SignedTransactionInfo,
    string? SignedRenewalInfo);
