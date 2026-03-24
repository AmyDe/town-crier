using System.Text.Json;
using TownCrier.Domain.Geocoding;
using TownCrier.Domain.UserProfiles;
using TownCrier.Infrastructure.Cosmos;

namespace TownCrier.Infrastructure.Tests.Cosmos;

public sealed class SystemTextJsonCosmosSerializerTests
{
    private readonly SystemTextJsonCosmosSerializer serializer;

    public SystemTextJsonCosmosSerializerTests()
    {
        var options = new JsonSerializerOptions
        {
            PropertyNamingPolicy = JsonNamingPolicy.CamelCase,
        };
        options.TypeInfoResolverChain.Add(CosmosJsonSerializerContext.Default);
        this.serializer = new SystemTextJsonCosmosSerializer(options);
    }

    [Test]
    public async Task Should_RoundTripCoordinates()
    {
        // Arrange
        var original = new Coordinates(51.5074, -0.1278);

        // Act
        using var stream = this.serializer.ToStream(original);
        var deserialized = this.serializer.FromStream<Coordinates>(stream);

        // Assert
        await Assert.That(deserialized.Latitude).IsEqualTo(original.Latitude);
        await Assert.That(deserialized.Longitude).IsEqualTo(original.Longitude);
    }

    [Test]
    public async Task Should_RoundTripNotificationPreferences()
    {
        // Arrange
        var original = new NotificationPreferences(PushEnabled: true, DigestDay: DayOfWeek.Wednesday);

        // Act
        using var stream = this.serializer.ToStream(original);
        var deserialized = this.serializer.FromStream<NotificationPreferences>(stream);

        // Assert
        await Assert.That(deserialized.PushEnabled).IsEqualTo(original.PushEnabled);
        await Assert.That(deserialized.DigestDay).IsEqualTo(original.DigestDay);
    }

    [Test]
    public async Task Should_RoundTripZoneNotificationPreferences()
    {
        // Arrange
        var original = new ZoneNotificationPreferences(
            NewApplications: true,
            StatusChanges: false,
            DecisionUpdates: true);

        // Act
        using var stream = this.serializer.ToStream(original);
        var deserialized = this.serializer.FromStream<ZoneNotificationPreferences>(stream);

        // Assert
        await Assert.That(deserialized.NewApplications).IsEqualTo(original.NewApplications);
        await Assert.That(deserialized.StatusChanges).IsEqualTo(original.StatusChanges);
        await Assert.That(deserialized.DecisionUpdates).IsEqualTo(original.DecisionUpdates);
    }

    [Test]
    public async Task Should_ReturnReadableStream_When_DeserializingAsStream()
    {
        // Arrange
        var original = new Coordinates(51.5074, -0.1278);
        var inputStream = this.serializer.ToStream(original);

        // Act
        var resultStream = this.serializer.FromStream<Stream>(inputStream);

        // Assert — stream must not be disposed and must be readable
        await Assert.That(resultStream.CanRead).IsTrue();

        using var reader = new StreamReader(resultStream);
        var json = await reader.ReadToEndAsync();
        await Assert.That(json).Contains("\"latitude\"");
    }

    [Test]
    public async Task Should_UseCamelCasePropertyNames_When_Serializing()
    {
        // Arrange
        var coordinates = new Coordinates(51.5074, -0.1278);

        // Act
        using var stream = this.serializer.ToStream(coordinates);
        using var reader = new StreamReader(stream);
        var json = await reader.ReadToEndAsync();

        // Assert
        await Assert.That(json).Contains("\"latitude\"");
        await Assert.That(json).Contains("\"longitude\"");
    }
}
