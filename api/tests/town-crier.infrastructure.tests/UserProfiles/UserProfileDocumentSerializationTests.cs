using System.Text.Json;
using TownCrier.Domain.UserProfiles;
using TownCrier.Infrastructure.Cosmos;
using TownCrier.Infrastructure.UserProfiles;

namespace TownCrier.Infrastructure.Tests.UserProfiles;

public sealed class UserProfileDocumentSerializationTests
{
    private readonly SystemTextJsonCosmosSerializer serializer;

    public UserProfileDocumentSerializationTests()
    {
        var options = new JsonSerializerOptions
        {
            PropertyNamingPolicy = JsonNamingPolicy.CamelCase,
        };
        options.TypeInfoResolverChain.Add(CosmosJsonSerializerContext.Default);
        this.serializer = new SystemTextJsonCosmosSerializer(options);
    }

    [Test]
    public async Task Should_RoundTripUserProfileDocument_When_Serialized()
    {
        // Arrange
        var profile = UserProfile.Register("auth0|user-1");
        profile.UpdatePreferences("SW1A 1AA", new NotificationPreferences(true, DayOfWeek.Wednesday));
        profile.ActivateSubscription(SubscriptionTier.Pro, new DateTimeOffset(2027, 1, 1, 0, 0, 0, TimeSpan.Zero));
        profile.LinkOriginalTransactionId("txn-ser-123");
        profile.SetZonePreferences("zone-1", new ZoneNotificationPreferences(true, true, false));
        var original = UserProfileDocument.FromDomain(profile);

        // Act
        using var stream = this.serializer.ToStream(original);
        var deserialized = this.serializer.FromStream<UserProfileDocument>(stream);

        // Assert
        await Assert.That(deserialized.Id).IsEqualTo(original.Id);
        await Assert.That(deserialized.UserId).IsEqualTo(original.UserId);
        await Assert.That(deserialized.Postcode).IsEqualTo(original.Postcode);
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
        using var stream = this.serializer.ToStream(document);
        using var reader = new StreamReader(stream);
        var json = await reader.ReadToEndAsync();

        // Assert
        await Assert.That(json).Contains("\"id\"");
        await Assert.That(json).Contains("\"userId\"");
        await Assert.That(json).Contains("\"pushEnabled\"");
        await Assert.That(json).Contains("\"tier\"");
        await Assert.That(json).Contains("\"zonePreferences\"");
    }
}
