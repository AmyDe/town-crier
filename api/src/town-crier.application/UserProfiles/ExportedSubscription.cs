using TownCrier.Domain.UserProfiles;

namespace TownCrier.Application.UserProfiles;

public sealed record ExportedSubscription(
    SubscriptionTier Tier,
    DateTimeOffset? ExpiresAt,
    string? OriginalTransactionId,
    DateTimeOffset? GracePeriodExpiresAt);
