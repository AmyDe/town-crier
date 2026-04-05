using System.Text.Json;
using TownCrier.Domain.UserProfiles;
using TownCrier.Infrastructure.Cosmos;
using TownCrier.Infrastructure.UserProfiles;

namespace TownCrier.Infrastructure.Tests.UserProfiles;

public sealed class UserProfileDocumentSerializationTests
{
    private readonly JsonSerializerOptions jsonOptions;

    public UserProfileDocumentSerializationTests()
    {
        this.jsonOptions = new JsonSerializerOptions
        {
            PropertyNamingPolicy = JsonNamingPolicy.CamelCase,
        };
        this.jsonOptions.TypeInfoResolverChain.Add(CosmosJsonSerializerContext.Default);
    }

    [Test]
    public async Task Should_RoundTripUserProfileDocument_When_Serialized()
    {
        // Arrange
        var profile = UserProfile.Register("auth0|user-1");
        profile.UpdatePreferences(new NotificationPreferences(true, DayOfWeek.Wednesday));
        profile.ActivateSubscription(SubscriptionTier.Pro, new DateTimeOffset(2027, 1, 1, 0, 0, 0, TimeSpan.Zero));
        profile.LinkOriginalTransactionId("txn-ser-123");
        profile.SetZonePreferences("zone-1", new ZoneNotificationPreferences(true, true, false));
        var original = UserProfileDocument.FromDomain(profile);

        // Act
        var json = JsonSerializer.Serialize(original, this.jsonOptions);
        var deserialized = JsonSerializer.Deserialize<UserProfileDocument>(json, this.jsonOptions)!;

        // Assert
        await Assert.That(deserialized.Id).IsEqualTo(original.Id);
        await Assert.That(deserialized.UserId).IsEqualTo(original.UserId);
        await Assert.That(deserialized.PushEnabled).IsEqualTo(original.PushEnabled);
        await Assert.That(deserialized.DigestDay).IsEqualTo(original.DigestDay);
        await Assert.That(deserialized.Tier).IsEqualTo(original.Tier);
        await Assert.That(deserialized.SubscriptionExpiry).IsEqualTo(original.SubscriptionExpiry);
        await Assert.That(deserialized.OriginalTransactionId).IsEqualTo(original.OriginalTransactionId);
        await Assert.That(deserialized.ZonePreferences).HasCount().EqualTo(1);
    }

    [Test]
    public async Task Should_UseCamelCasePropertyNames_When_Serialized()
    {
        // Arrange
        var profile = UserProfile.Register("auth0|user-1");
        var document = UserProfileDocument.FromDomain(profile);

        // Act
        var json = JsonSerializer.Serialize(document, this.jsonOptions);

        // Assert
        await Assert.That(json).Contains("\"id\"");
        await Assert.That(json).Contains("\"userId\"");
        await Assert.That(json).Contains("\"pushEnabled\"");
        await Assert.That(json).Contains("\"tier\"");
        await Assert.That(json).Contains("\"zonePreferences\"");
    }
}
