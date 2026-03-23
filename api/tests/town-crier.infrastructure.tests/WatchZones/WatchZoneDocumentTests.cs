using System.Text.Json;
using TownCrier.Domain.Geocoding;
using TownCrier.Domain.WatchZones;
using TownCrier.Infrastructure.Cosmos;
using TownCrier.Infrastructure.WatchZones;

namespace TownCrier.Infrastructure.Tests.WatchZones;

public sealed class WatchZoneDocumentTests
{
    [Test]
    public async Task Should_PreserveAllFields_When_MappedFromDomain()
    {
        // Arrange
        var zone = new WatchZone("zone-1", "user-1", "Home Area", new Coordinates(51.5074, -0.1278), 5000, 42);

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
    }

    [Test]
    public async Task Should_RoundTripToDomain_When_MappedBackAndForth()
    {
        // Arrange
        var original = new WatchZone("zone-1", "user-1", "Home Area", new Coordinates(51.5074, -0.1278), 5000, 42);

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
    }

    [Test]
    public async Task Should_RoundTripThroughJsonSerialization_When_UsingCosmosSerializer()
    {
        // Arrange
        var original = WatchZoneDocument.FromDomain(
            new WatchZone("zone-1", "user-1", "Home Area", new Coordinates(51.5074, -0.1278), 5000, 42));

        var jsonOptions = new JsonSerializerOptions
        {
            PropertyNamingPolicy = JsonNamingPolicy.CamelCase,
        };
        jsonOptions.TypeInfoResolverChain.Add(CosmosJsonSerializerContext.Default);

        var serializer = new SystemTextJsonCosmosSerializer(jsonOptions);

        // Act
        using var stream = serializer.ToStream(original);
        var deserialized = serializer.FromStream<WatchZoneDocument>(stream);

        // Assert
        await Assert.That(deserialized.Id).IsEqualTo(original.Id);
        await Assert.That(deserialized.UserId).IsEqualTo(original.UserId);
        await Assert.That(deserialized.Name).IsEqualTo(original.Name);
        await Assert.That(deserialized.Latitude).IsEqualTo(original.Latitude);
        await Assert.That(deserialized.Longitude).IsEqualTo(original.Longitude);
        await Assert.That(deserialized.RadiusMetres).IsEqualTo(original.RadiusMetres);
        await Assert.That(deserialized.AuthorityId).IsEqualTo(original.AuthorityId);
    }
}
