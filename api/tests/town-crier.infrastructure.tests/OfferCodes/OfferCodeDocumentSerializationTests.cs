using System.Text.Json;
using TownCrier.Domain.OfferCodes;
using TownCrier.Domain.UserProfiles;
using TownCrier.Infrastructure.Cosmos;
using TownCrier.Infrastructure.OfferCodes;

namespace TownCrier.Infrastructure.Tests.OfferCodes;

public sealed class OfferCodeDocumentSerializationTests
{
    private readonly JsonSerializerOptions jsonOptions;

    public OfferCodeDocumentSerializationTests()
    {
        this.jsonOptions = new JsonSerializerOptions
        {
            PropertyNamingPolicy = JsonNamingPolicy.CamelCase,
        };
        this.jsonOptions.TypeInfoResolverChain.Add(CosmosJsonSerializerContext.Default);
    }

    [Test]
    public async Task Should_RoundTripOfferCodeDocument_When_Serialized()
    {
        // Arrange
        var code = new OfferCode(
            "A7KMZQR3FNXP",
            SubscriptionTier.Pro,
            30,
            new DateTimeOffset(2026, 4, 18, 12, 0, 0, TimeSpan.Zero));
        code.Redeem("auth0|user-42", new DateTimeOffset(2026, 4, 19, 9, 0, 0, TimeSpan.Zero));
        var original = OfferCodeDocument.FromDomain(code);

        // Act
        var json = JsonSerializer.Serialize(original, this.jsonOptions);
        var deserialized = JsonSerializer.Deserialize<OfferCodeDocument>(json, this.jsonOptions)!;

        // Assert
        await Assert.That(deserialized.Id).IsEqualTo(original.Id);
        await Assert.That(deserialized.Code).IsEqualTo(original.Code);
        await Assert.That(deserialized.Tier).IsEqualTo(original.Tier);
        await Assert.That(deserialized.DurationDays).IsEqualTo(original.DurationDays);
        await Assert.That(deserialized.CreatedAt).IsEqualTo(original.CreatedAt);
        await Assert.That(deserialized.RedeemedByUserId).IsEqualTo(original.RedeemedByUserId);
        await Assert.That(deserialized.RedeemedAt).IsEqualTo(original.RedeemedAt);
    }

    [Test]
    public async Task Should_UseCamelCasePropertyNames_When_Serialized()
    {
        // Arrange
        var code = new OfferCode(
            "BBBBBBBBBBBB",
            SubscriptionTier.Personal,
            14,
            new DateTimeOffset(2026, 4, 18, 12, 0, 0, TimeSpan.Zero));
        var document = OfferCodeDocument.FromDomain(code);

        // Act
        var json = JsonSerializer.Serialize(document, this.jsonOptions);

        // Assert
        await Assert.That(json).Contains("\"id\"");
        await Assert.That(json).Contains("\"code\"");
        await Assert.That(json).Contains("\"tier\"");
        await Assert.That(json).Contains("\"durationDays\"");
        await Assert.That(json).Contains("\"createdAt\"");
    }
}
