using TownCrier.Domain.UserProfiles;

namespace TownCrier.Domain.Entitlements;

public static class EntitlementMap
{
    private static readonly IReadOnlySet<Entitlement> EmptySet =
        new HashSet<Entitlement>();

    private static readonly IReadOnlySet<Entitlement> PersonalEntitlements =
        new HashSet<Entitlement>
        {
            Entitlement.InstantEmails,
            Entitlement.StatusChangeAlerts,
            Entitlement.DecisionUpdateAlerts,
        };

    private static readonly IReadOnlySet<Entitlement> ProEntitlements =
        new HashSet<Entitlement>
        {
            Entitlement.InstantEmails,
            Entitlement.SearchApplications,
            Entitlement.StatusChangeAlerts,
            Entitlement.DecisionUpdateAlerts,
        };

    public static IReadOnlySet<Entitlement> EntitlementsFor(SubscriptionTier tier) => tier switch
    {
        SubscriptionTier.Personal => PersonalEntitlements,
        SubscriptionTier.Pro => ProEntitlements,
        _ => EmptySet,
    };

    public static int LimitFor(SubscriptionTier tier, Quota quota) => (tier, quota) switch
    {
        (SubscriptionTier.Free, Quota.WatchZones) => 1,
        (SubscriptionTier.Personal, Quota.WatchZones) => 3,
        (SubscriptionTier.Pro, Quota.WatchZones) => int.MaxValue,
        _ => 1,
    };
}
