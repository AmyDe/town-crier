using TownCrier.Domain.Entitlements;
using TownCrier.Domain.UserProfiles;

namespace TownCrier.Domain.Tests.Entitlements;

public sealed class EntitlementMapTests
{
    [Test]
    public async Task FreeTier_Should_HaveNoEntitlements()
    {
        var entitlements = EntitlementMap.EntitlementsFor(SubscriptionTier.Free);

        await Assert.That(entitlements).HasCount().EqualTo(0);
    }

    [Test]
    public async Task PersonalTier_Should_HaveInstantEmailsAndAlerts()
    {
        var entitlements = EntitlementMap.EntitlementsFor(SubscriptionTier.Personal);

        await Assert.That(entitlements).Contains(Entitlement.InstantEmails);
        await Assert.That(entitlements).Contains(Entitlement.StatusChangeAlerts);
        await Assert.That(entitlements).Contains(Entitlement.DecisionUpdateAlerts);
        await Assert.That(entitlements).DoesNotContain(Entitlement.SearchApplications);
    }

    [Test]
    public async Task ProTier_Should_HaveAllEntitlements()
    {
        var entitlements = EntitlementMap.EntitlementsFor(SubscriptionTier.Pro);

        await Assert.That(entitlements).Contains(Entitlement.InstantEmails);
        await Assert.That(entitlements).Contains(Entitlement.SearchApplications);
        await Assert.That(entitlements).Contains(Entitlement.StatusChangeAlerts);
        await Assert.That(entitlements).Contains(Entitlement.DecisionUpdateAlerts);
    }

    [Test]
    public async Task FreeTier_WatchZoneLimit_Should_Be1()
    {
        var limit = EntitlementMap.LimitFor(SubscriptionTier.Free, Quota.WatchZones);

        await Assert.That(limit).IsEqualTo(1);
    }

    [Test]
    public async Task PersonalTier_WatchZoneLimit_Should_Be3()
    {
        var limit = EntitlementMap.LimitFor(SubscriptionTier.Personal, Quota.WatchZones);

        await Assert.That(limit).IsEqualTo(3);
    }

    [Test]
    public async Task ProTier_WatchZoneLimit_Should_BeUnlimited()
    {
        var limit = EntitlementMap.LimitFor(SubscriptionTier.Pro, Quota.WatchZones);

        await Assert.That(limit).IsEqualTo(int.MaxValue);
    }
}
