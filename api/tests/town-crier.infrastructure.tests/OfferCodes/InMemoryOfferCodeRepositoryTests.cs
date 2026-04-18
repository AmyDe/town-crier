using TownCrier.Domain.OfferCodes;
using TownCrier.Domain.UserProfiles;
using TownCrier.Infrastructure.OfferCodes;

namespace TownCrier.Infrastructure.Tests.OfferCodes;

public sealed class InMemoryOfferCodeRepositoryTests
{
    [Test]
    public async Task Should_ReturnNull_When_CodeDoesNotExist()
    {
        // Arrange
        var repo = new InMemoryOfferCodeRepository();

        // Act
        var result = await repo.GetAsync("A7KMZQR3FNXP", CancellationToken.None);

        // Assert
        await Assert.That(result).IsNull();
    }

    [Test]
    public async Task Should_StoreOfferCode_When_Created()
    {
        // Arrange
        var repo = new InMemoryOfferCodeRepository();
        var code = new OfferCode(
            "A7KMZQR3FNXP",
            SubscriptionTier.Pro,
            30,
            new DateTimeOffset(2026, 4, 18, 12, 0, 0, TimeSpan.Zero));

        // Act
        await repo.CreateAsync(code, CancellationToken.None);

        // Assert
        var fetched = await repo.GetAsync("A7KMZQR3FNXP", CancellationToken.None);
        await Assert.That(fetched).IsNotNull();
        await Assert.That(fetched!.Tier).IsEqualTo(SubscriptionTier.Pro);
    }

    [Test]
    public async Task Should_Throw_When_CreatingDuplicateCode()
    {
        // Arrange
        var repo = new InMemoryOfferCodeRepository();
        var code = new OfferCode(
            "A7KMZQR3FNXP",
            SubscriptionTier.Pro,
            30,
            new DateTimeOffset(2026, 4, 18, 12, 0, 0, TimeSpan.Zero));
        await repo.CreateAsync(code, CancellationToken.None);

        // Act & Assert
        await Assert.ThrowsAsync<InvalidOperationException>(
            () => repo.CreateAsync(code, CancellationToken.None));
    }

    [Test]
    public async Task Should_PersistRedeemedState_When_Saved()
    {
        // Arrange
        var repo = new InMemoryOfferCodeRepository();
        var code = new OfferCode(
            "BBBBBBBBBBBB",
            SubscriptionTier.Personal,
            14,
            new DateTimeOffset(2026, 4, 18, 12, 0, 0, TimeSpan.Zero));
        await repo.CreateAsync(code, CancellationToken.None);

        // Act
        code.Redeem("auth0|user-1", new DateTimeOffset(2026, 4, 19, 9, 0, 0, TimeSpan.Zero));
        await repo.SaveAsync(code, CancellationToken.None);

        // Assert
        var fetched = await repo.GetAsync("BBBBBBBBBBBB", CancellationToken.None);
        await Assert.That(fetched!.IsRedeemed).IsTrue();
        await Assert.That(fetched.RedeemedByUserId).IsEqualTo("auth0|user-1");
    }
}
