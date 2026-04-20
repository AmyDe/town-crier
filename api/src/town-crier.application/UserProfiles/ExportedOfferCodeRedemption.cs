using TownCrier.Domain.UserProfiles;

namespace TownCrier.Application.UserProfiles;

public sealed record ExportedOfferCodeRedemption(
    string Code,
    SubscriptionTier Tier,
    int DurationDays,
    DateTimeOffset RedeemedAt);
