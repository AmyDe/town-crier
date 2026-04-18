using TownCrier.Domain.OfferCodes;
using TownCrier.Domain.UserProfiles;
using TownCrier.Infrastructure.OfferCodes;
using TownCrier.Infrastructure.Tests.Cosmos;

namespace TownCrier.Infrastructure.Tests.OfferCodes;

public sealed class CosmosOfferCodeRepositoryTests
{
    [Test]
    public async Task Should_ReturnNull_When_CodeDoesNotExist()
    {
        // Arrange
        var client = new FakeCosmosRestClient();
        var repo = new CosmosOfferCodeRepository(client);

        // Act
        var result = await repo.GetAsync("A7KMZQR3FNXP", CancellationToken.None);

        // Assert
        await Assert.That(result).IsNull();
    }

    [Test]
    public async Task Should_RoundTripOfferCode_When_Created()
    {
        // Arrange
        var client = new FakeCosmosRestClient();
        var repo = new CosmosOfferCodeRepository(client);
        var code = new OfferCode(
            "A7KMZQR3FNXP",
            SubscriptionTier.Pro,
            30,
            new DateTimeOffset(2026, 4, 18, 12, 0, 0, TimeSpan.Zero));

        // Act
        await repo.CreateAsync(code, CancellationToken.None);
        var fetched = await repo.GetAsync("A7KMZQR3FNXP", CancellationToken.None);

        // Assert
        await Assert.That(fetched).IsNotNull();
        await Assert.That(fetched!.Code).IsEqualTo("A7KMZQR3FNXP");
        await Assert.That(fetched.Tier).IsEqualTo(SubscriptionTier.Pro);
        await Assert.That(fetched.DurationDays).IsEqualTo(30);
        await Assert.That(fetched.IsRedeemed).IsFalse();
    }

    [Test]
    public async Task Should_PersistRedeemedState_When_Saved()
    {
        // Arrange
        var client = new FakeCosmosRestClient();
        var repo = new CosmosOfferCodeRepository(client);
        var code = new OfferCode(
            "BBBBBBBBBBBB",
            SubscriptionTier.Personal,
            14,
            new DateTimeOffset(2026, 4, 18, 12, 0, 0, TimeSpan.Zero));
        await repo.CreateAsync(code, CancellationToken.None);

        // Act
        var redeemedAt = new DateTimeOffset(2026, 4, 19, 9, 0, 0, TimeSpan.Zero);
        code.Redeem("auth0|user-99", redeemedAt);
        await repo.SaveAsync(code, CancellationToken.None);

        // Assert
        var fetched = await repo.GetAsync("BBBBBBBBBBBB", CancellationToken.None);
        await Assert.That(fetched).IsNotNull();
        await Assert.That(fetched!.IsRedeemed).IsTrue();
        await Assert.That(fetched.RedeemedByUserId).IsEqualTo("auth0|user-99");
        await Assert.That(fetched.RedeemedAt).IsEqualTo(redeemedAt);
    }

    [Test]
    public async Task Should_NotThrow_When_CreateAsyncCalledWithExistingCode()
    {
        // Arrange — UpsertDocumentAsync has no distinct "create" semantics; CreateAsync is best-effort
        // (last-writer-wins). See spec docs/specs/offer-codes.md § Race-condition handling.
        var client = new FakeCosmosRestClient();
        var repo = new CosmosOfferCodeRepository(client);
        var code = new OfferCode(
            "A7KMZQR3FNXP",
            SubscriptionTier.Pro,
            30,
            new DateTimeOffset(2026, 4, 18, 12, 0, 0, TimeSpan.Zero));

        // Act
        await repo.CreateAsync(code, CancellationToken.None);

        // Assert — calling CreateAsync again must not throw (treated as upsert)
        await repo.CreateAsync(code, CancellationToken.None);
        var fetched = await repo.GetAsync("A7KMZQR3FNXP", CancellationToken.None);
        await Assert.That(fetched).IsNotNull();
    }
}
