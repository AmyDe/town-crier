using TownCrier.Domain.UserProfiles;

namespace TownCrier.Application.OfferCodes;

public sealed record RedeemOfferCodeResult(SubscriptionTier Tier, DateTimeOffset ExpiresAt);
