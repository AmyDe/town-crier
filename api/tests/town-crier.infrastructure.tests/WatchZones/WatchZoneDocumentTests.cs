using System.Text.Json;
using TownCrier.Domain.Geocoding;
using TownCrier.Domain.WatchZones;
using TownCrier.Infrastructure.Cosmos;
using TownCrier.Infrastructure.WatchZones;

namespace TownCrier.Infrastructure.Tests.WatchZones;

public sealed class WatchZoneDocumentTests
{
    private static readonly DateTimeOffset TestCreatedAt = new(2026, 3, 15, 10, 0, 0, TimeSpan.Zero);

    [Test]
    public async Task Should_PreserveAllFields_When_MappedFromDomain()
    {
        // Arrange
        var zone = new WatchZone("zone-1", "user-1", "Home Area", new Coordinates(51.5074, -0.1278), 5000, 42, TestCreatedAt);

        // Act
        var document = WatchZoneDocument.FromDomain(zone);

        // Assert
        await Assert.That(document.Id).IsEqualTo("zone-1");
        await Assert.That(document.UserId).IsEqualTo("user-1");
        await Assert.That(document.Name).IsEqualTo("Home Area");
        await Assert.That(document.Latitude).IsEqualTo(51.5074);
        await Assert.That(document.Longitude).IsEqualTo(-0.1278);
        await Assert.That(document.RadiusMetres).IsEqualTo(5000);
        await Assert.That(document.AuthorityId).IsEqualTo(42);
        await Assert.That(document.CreatedAt).IsEqualTo(TestCreatedAt);
    }

    [Test]
    public async Task Should_RoundTripToDomain_When_MappedBackAndForth()
    {
        // Arrange
        var original = new WatchZone("zone-1", "user-1", "Home Area", new Coordinates(51.5074, -0.1278), 5000, 42, TestCreatedAt);

        // Act
        var document = WatchZoneDocument.FromDomain(original);
        var roundTripped = document.ToDomain();

        // Assert
        await Assert.That(roundTripped.Id).IsEqualTo(original.Id);
        await Assert.That(roundTripped.UserId).IsEqualTo(original.UserId);
        await Assert.That(roundTripped.Name).IsEqualTo(original.Name);
        await Assert.That(roundTripped.Centre.Latitude).IsEqualTo(original.Centre.Latitude);
        await Assert.That(roundTripped.Centre.Longitude).IsEqualTo(original.Centre.Longitude);
        await Assert.That(roundTripped.RadiusMetres).IsEqualTo(original.RadiusMetres);
        await Assert.That(roundTripped.AuthorityId).IsEqualTo(original.AuthorityId);
        await Assert.That(roundTripped.CreatedAt).IsEqualTo(original.CreatedAt);
    }

    [Test]
    public async Task Should_RoundTripThroughJsonSerialization_When_SerializedWithSourceGenerators()
    {
        // Arrange
        var original = WatchZoneDocument.FromDomain(
            new WatchZone("zone-1", "user-1", "Home Area", new Coordinates(51.5074, -0.1278), 5000, 42, TestCreatedAt));

        var jsonOptions = CreateJsonOptions();

        // Act
        var json = JsonSerializer.Serialize(original, jsonOptions);
        var deserialized = JsonSerializer.Deserialize<WatchZoneDocument>(json, jsonOptions)!;

        // Assert
        await Assert.That(deserialized.Id).IsEqualTo(original.Id);
        await Assert.That(deserialized.UserId).IsEqualTo(original.UserId);
        await Assert.That(deserialized.Name).IsEqualTo(original.Name);
        await Assert.That(deserialized.Latitude).IsEqualTo(original.Latitude);
        await Assert.That(deserialized.Longitude).IsEqualTo(original.Longitude);
        await Assert.That(deserialized.RadiusMetres).IsEqualTo(original.RadiusMetres);
        await Assert.That(deserialized.AuthorityId).IsEqualTo(original.AuthorityId);
        await Assert.That(deserialized.CreatedAt).IsEqualTo(original.CreatedAt);
    }

    [Test]
    public async Task Should_DefaultCreatedAtToMinValue_When_CosmosDocumentHasNoCreatedAt()
    {
        // Arrange — simulate a legacy document with no createdAt field
        var json = """{"id":"zone-1","userId":"user-1","name":"Old Zone","latitude":51.5,"longitude":-0.1,"radiusMetres":500,"authorityId":100}""";

        var jsonOptions = CreateJsonOptions();

        // Act
        var document = JsonSerializer.Deserialize<WatchZoneDocument>(json, jsonOptions)!;
        var zone = document.ToDomain();

        // Assert — missing createdAt should default to MinValue
        await Assert.That(zone.CreatedAt).IsEqualTo(DateTimeOffset.MinValue);
    }

    [Test]
    public async Task Should_PreserveNotificationFlags_When_MappedFromDomain()
    {
        // Arrange
        var zone = new WatchZone(
            "zone-1",
            "user-1",
            "Home",
            new Coordinates(51.5074, -0.1278),
            5000,
            42,
            TestCreatedAt,
            pushEnabled: false,
            emailInstantEnabled: false);

        // Act
        var document = WatchZoneDocument.FromDomain(zone);

        // Assert
        await Assert.That(document.PushEnabled).IsEqualTo(false);
        await Assert.That(document.EmailInstantEnabled).IsEqualTo(false);
    }

    [Test]
    public async Task Should_RoundTripNotificationFlags_When_MappedBackAndForth()
    {
        // Arrange
        var original = new WatchZone(
            "zone-1",
            "user-1",
            "Home",
            new Coordinates(51.5074, -0.1278),
            5000,
            42,
            TestCreatedAt,
            pushEnabled: false,
            emailInstantEnabled: true);

        // Act
        var document = WatchZoneDocument.FromDomain(original);
        var roundTripped = document.ToDomain();

        // Assert
        await Assert.That(roundTripped.PushEnabled).IsFalse();
        await Assert.That(roundTripped.EmailInstantEnabled).IsTrue();
    }

    [Test]
    public async Task Should_DefaultPushEnabledToTrue_When_CosmosDocumentHasNoPushEnabled()
    {
        // Arrange — simulate a legacy document predating the per-zone notification flags
        var json = """{"id":"zone-1","userId":"user-1","name":"Old Zone","latitude":51.5,"longitude":-0.1,"radiusMetres":500,"authorityId":100,"createdAt":"2026-01-01T00:00:00+00:00"}""";

        var jsonOptions = CreateJsonOptions();

        // Act
        var document = JsonSerializer.Deserialize<WatchZoneDocument>(json, jsonOptions)!;
        var zone = document.ToDomain();

        // Assert — missing pushEnabled hydrates to true (preserves current behaviour for existing zones)
        await Assert.That(zone.PushEnabled).IsTrue();
    }

    [Test]
    public async Task Should_DefaultEmailInstantEnabledToTrue_When_CosmosDocumentHasNoEmailInstantEnabled()
    {
        // Arrange — simulate a legacy document predating the per-zone notification flags
        var json = """{"id":"zone-1","userId":"user-1","name":"Old Zone","latitude":51.5,"longitude":-0.1,"radiusMetres":500,"authorityId":100,"createdAt":"2026-01-01T00:00:00+00:00"}""";

        var jsonOptions = CreateJsonOptions();

        // Act
        var document = JsonSerializer.Deserialize<WatchZoneDocument>(json, jsonOptions)!;
        var zone = document.ToDomain();

        // Assert — missing emailInstantEnabled hydrates to true (preserves current behaviour for existing zones)
        await Assert.That(zone.EmailInstantEnabled).IsTrue();
    }

    [Test]
    public async Task Should_RoundTripNotificationFlagsThroughJson_When_SerializedWithSourceGenerators()
    {
        // Arrange
        var original = WatchZoneDocument.FromDomain(
            new WatchZone(
                "zone-1",
                "user-1",
                "Home",
                new Coordinates(51.5074, -0.1278),
                5000,
                42,
                TestCreatedAt,
                pushEnabled: false,
                emailInstantEnabled: false));

        var jsonOptions = CreateJsonOptions();

        // Act
        var json = JsonSerializer.Serialize(original, jsonOptions);
        var deserialized = JsonSerializer.Deserialize<WatchZoneDocument>(json, jsonOptions)!;

        // Assert
        await Assert.That(deserialized.PushEnabled).IsEqualTo(false);
        await Assert.That(deserialized.EmailInstantEnabled).IsEqualTo(false);
    }

    private static JsonSerializerOptions CreateJsonOptions()
    {
        var options = new JsonSerializerOptions
        {
            PropertyNamingPolicy = JsonNamingPolicy.CamelCase,
        };
        options.TypeInfoResolverChain.Add(CosmosJsonSerializerContext.Default);
        return options;
    }
}
