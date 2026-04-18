using TownCrier.Domain.OfferCodes;
using TownCrier.Domain.UserProfiles;
using TownCrier.Infrastructure.OfferCodes;

namespace TownCrier.Infrastructure.Tests.OfferCodes;

public sealed class OfferCodeDocumentTests
{
    [Test]
    public async Task Should_SetCodeAsId_When_MappedFromDomain()
    {
        // Arrange
        var code = new OfferCode(
            "A7KMZQR3FNXP",
            SubscriptionTier.Pro,
            30,
            new DateTimeOffset(2026, 4, 18, 12, 0, 0, TimeSpan.Zero));

        // Act
        var document = OfferCodeDocument.FromDomain(code);

        // Assert
        await Assert.That(document.Id).IsEqualTo("A7KMZQR3FNXP");
        await Assert.That(document.Code).IsEqualTo("A7KMZQR3FNXP");
    }

    [Test]
    public async Task Should_PreserveBasicFields_When_MappedFromDomain()
    {
        // Arrange
        var createdAt = new DateTimeOffset(2026, 4, 18, 12, 0, 0, TimeSpan.Zero);
        var code = new OfferCode("A7KMZQR3FNXP", SubscriptionTier.Pro, 30, createdAt);

        // Act
        var document = OfferCodeDocument.FromDomain(code);

        // Assert
        await Assert.That(document.Tier).IsEqualTo("Pro");
        await Assert.That(document.DurationDays).IsEqualTo(30);
        await Assert.That(document.CreatedAt).IsEqualTo(createdAt);
        await Assert.That(document.RedeemedByUserId).IsNull();
        await Assert.That(document.RedeemedAt).IsNull();
    }

    [Test]
    public async Task Should_RoundTripUnredeemedCode_When_MappedBackAndForth()
    {
        // Arrange
        var original = new OfferCode(
            "BBBBBBBBBBBB",
            SubscriptionTier.Personal,
            14,
            new DateTimeOffset(2026, 4, 18, 12, 0, 0, TimeSpan.Zero));

        // Act
        var document = OfferCodeDocument.FromDomain(original);
        var roundTripped = document.ToDomain();

        // Assert
        await Assert.That(roundTripped.Code).IsEqualTo(original.Code);
        await Assert.That(roundTripped.Tier).IsEqualTo(original.Tier);
        await Assert.That(roundTripped.DurationDays).IsEqualTo(original.DurationDays);
        await Assert.That(roundTripped.CreatedAt).IsEqualTo(original.CreatedAt);
        await Assert.That(roundTripped.IsRedeemed).IsFalse();
    }

    [Test]
    public async Task Should_RoundTripRedeemedCode_When_MappedBackAndForth()
    {
        // Arrange
        var original = new OfferCode(
            "A7KMZQR3FNXP",
            SubscriptionTier.Pro,
            30,
            new DateTimeOffset(2026, 4, 18, 12, 0, 0, TimeSpan.Zero));
        var redeemedAt = new DateTimeOffset(2026, 4, 19, 9, 30, 0, TimeSpan.Zero);
        original.Redeem("auth0|user-99", redeemedAt);

        // Act
        var document = OfferCodeDocument.FromDomain(original);
        var roundTripped = document.ToDomain();

        // Assert
        await Assert.That(roundTripped.IsRedeemed).IsTrue();
        await Assert.That(roundTripped.RedeemedByUserId).IsEqualTo("auth0|user-99");
        await Assert.That(roundTripped.RedeemedAt).IsEqualTo(redeemedAt);
    }
}
