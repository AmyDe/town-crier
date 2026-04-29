using TownCrier.Domain.Geocoding;
using TownCrier.Domain.WatchZones;

namespace TownCrier.Domain.Tests.WatchZones;

public sealed class WatchZoneTests
{
    private static readonly DateTimeOffset CreatedAt = new(2026, 4, 28, 12, 0, 0, TimeSpan.Zero);

    [Test]
    public async Task Should_DefaultPushEnabledToTrue_When_ConstructedWithoutFlag()
    {
        // Arrange & Act — preserves current behaviour: a new zone is opt-in to push.
        var zone = new WatchZone(
            "zone-1",
            "user-1",
            "Home",
            new Coordinates(51.5074, -0.1278),
            5000,
            42,
            CreatedAt);

        // Assert
        await Assert.That(zone.PushEnabled).IsTrue();
    }

    [Test]
    public async Task Should_DefaultEmailInstantEnabledToTrue_When_ConstructedWithoutFlag()
    {
        // Arrange & Act — preserves current behaviour: a new zone is opt-in to instant email.
        var zone = new WatchZone(
            "zone-1",
            "user-1",
            "Home",
            new Coordinates(51.5074, -0.1278),
            5000,
            42,
            CreatedAt);

        // Assert
        await Assert.That(zone.EmailInstantEnabled).IsTrue();
    }

    [Test]
    public async Task Should_StoreNotificationFlags_When_ConstructedWithExplicitValues()
    {
        // Arrange & Act
        var zone = new WatchZone(
            "zone-1",
            "user-1",
            "Home",
            new Coordinates(51.5074, -0.1278),
            5000,
            42,
            CreatedAt,
            pushEnabled: false,
            emailInstantEnabled: false);

        // Assert
        await Assert.That(zone.PushEnabled).IsFalse();
        await Assert.That(zone.EmailInstantEnabled).IsFalse();
    }

    [Test]
    public async Task Should_UpdatePushEnabled_When_WithUpdatesProvidesNewValue()
    {
        // Arrange
        var zone = new WatchZone(
            "zone-1",
            "user-1",
            "Home",
            new Coordinates(51.5074, -0.1278),
            5000,
            42,
            CreatedAt,
            pushEnabled: true,
            emailInstantEnabled: true);

        // Act
        var updated = zone.WithUpdates(pushEnabled: false);

        // Assert — push toggled off, other flag preserved
        await Assert.That(updated.PushEnabled).IsFalse();
        await Assert.That(updated.EmailInstantEnabled).IsTrue();
    }

    [Test]
    public async Task Should_UpdateEmailInstantEnabled_When_WithUpdatesProvidesNewValue()
    {
        // Arrange
        var zone = new WatchZone(
            "zone-1",
            "user-1",
            "Home",
            new Coordinates(51.5074, -0.1278),
            5000,
            42,
            CreatedAt,
            pushEnabled: true,
            emailInstantEnabled: true);

        // Act
        var updated = zone.WithUpdates(emailInstantEnabled: false);

        // Assert — email toggled off, other flag preserved
        await Assert.That(updated.EmailInstantEnabled).IsFalse();
        await Assert.That(updated.PushEnabled).IsTrue();
    }

    [Test]
    public async Task Should_PreserveBothFlags_When_WithUpdatesProvidesNeither()
    {
        // Arrange
        var zone = new WatchZone(
            "zone-1",
            "user-1",
            "Home",
            new Coordinates(51.5074, -0.1278),
            5000,
            42,
            CreatedAt,
            pushEnabled: false,
            emailInstantEnabled: false);

        // Act
        var updated = zone.WithUpdates(name: "Renamed");

        // Assert
        await Assert.That(updated.PushEnabled).IsFalse();
        await Assert.That(updated.EmailInstantEnabled).IsFalse();
    }
}
