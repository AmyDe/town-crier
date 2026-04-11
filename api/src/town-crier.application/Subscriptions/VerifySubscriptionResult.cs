using TownCrier.Domain.Entitlements;
using TownCrier.Domain.UserProfiles;

namespace TownCrier.Application.Subscriptions;

public sealed record VerifySubscriptionResult(
    SubscriptionTier Tier,
    DateTimeOffset? SubscriptionExpiry,
    IReadOnlyList<string> Entitlements,
    int WatchZoneLimit)
{
    public static VerifySubscriptionResult FromProfile(UserProfile profile)
    {
        ArgumentNullException.ThrowIfNull(profile);

        var tier = profile.Tier;
        var entitlements = EntitlementMap.EntitlementsFor(tier)
            .Select(e => e.ToString())
            .ToList();
        var watchZoneLimit = EntitlementMap.LimitFor(tier, Quota.WatchZones);

        return new VerifySubscriptionResult(tier, profile.SubscriptionExpiry, entitlements, watchZoneLimit);
    }
}
