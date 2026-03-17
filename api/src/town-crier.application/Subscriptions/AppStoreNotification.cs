using TownCrier.Domain.UserProfiles;

namespace TownCrier.Application.Subscriptions;

public sealed record AppStoreNotification(
    AppStoreNotificationType NotificationType,
    string OriginalTransactionId,
    SubscriptionTier Tier,
    DateTimeOffset? ExpiresDate);
