using TownCrier.Domain.Subscriptions;
using TownCrier.Domain.UserProfiles;

namespace TownCrier.Application.Tests.Subscriptions;

public sealed class ProductMappingTests
{
    [Test]
    public async Task Should_ReturnPersonalTier_When_PersonalMonthlyProductId()
    {
        var tier = ProductMapping.ToTier("uk.co.towncrier.personal.monthly");

        await Assert.That(tier).IsEqualTo(SubscriptionTier.Personal);
    }

    [Test]
    public async Task Should_ReturnProTier_When_ProMonthlyProductId()
    {
        var tier = ProductMapping.ToTier("uk.co.towncrier.pro.monthly");

        await Assert.That(tier).IsEqualTo(SubscriptionTier.Pro);
    }

    [Test]
    public async Task Should_ThrowArgumentException_When_UnknownProductId()
    {
        await Assert.ThrowsAsync<ArgumentException>(
            () => Task.FromResult(ProductMapping.ToTier("com.unknown.product")));
    }
}
