using TownCrier.Domain.DeviceRegistrations;
using TownCrier.Infrastructure.DeviceRegistrations;

namespace TownCrier.Infrastructure.Tests.DeviceRegistrations;

public sealed class DeviceRegistrationDocumentTests
{
    [Test]
    public async Task Should_RoundTripToDomainAndBack_When_ValidRegistration()
    {
        // Arrange
        var original = DeviceRegistration.Create(
            "auth0|user-123",
            "apns-token-abc123",
            DevicePlatform.Ios,
            new DateTimeOffset(2026, 3, 17, 10, 0, 0, TimeSpan.Zero));

        // Act
        var document = DeviceRegistrationDocument.FromDomain(original);
        var restored = document.ToDomain();

        // Assert
        await Assert.That(restored.UserId).IsEqualTo(original.UserId);
        await Assert.That(restored.Token).IsEqualTo(original.Token);
        await Assert.That(restored.Platform).IsEqualTo(original.Platform);
        await Assert.That(restored.RegisteredAt).IsEqualTo(original.RegisteredAt);
    }

    [Test]
    public async Task Should_UseTokenAsDocumentId_When_ConvertingFromDomain()
    {
        // Arrange
        var registration = DeviceRegistration.Create(
            "auth0|user-456",
            "fcm-token-xyz789",
            DevicePlatform.Android,
            new DateTimeOffset(2026, 3, 17, 12, 0, 0, TimeSpan.Zero));

        // Act
        var document = DeviceRegistrationDocument.FromDomain(registration);

        // Assert
        await Assert.That(document.Id).IsEqualTo("fcm-token-xyz789");
        await Assert.That(document.UserId).IsEqualTo("auth0|user-456");
        await Assert.That(document.Platform).IsEqualTo("Android");
    }

    [Test]
    public async Task Should_SetUserIdAsPartitionKeyValue_When_ConvertingFromDomain()
    {
        // Arrange
        var registration = DeviceRegistration.Create(
            "auth0|user-789",
            "apns-token-device1",
            DevicePlatform.Ios,
            new DateTimeOffset(2026, 3, 17, 14, 0, 0, TimeSpan.Zero));

        // Act
        var document = DeviceRegistrationDocument.FromDomain(registration);

        // Assert
        await Assert.That(document.UserId).IsEqualTo("auth0|user-789");
    }

    [Test]
    public async Task Should_SetTtlTo180Days_When_ConvertingFromDomain()
    {
        // Arrange — Cosmos TTL is an integer number of seconds. Devices that stop
        // refreshing (app uninstalled, logged out) are purged after 180 days so the
        // push token store doesn't accumulate permanently-stale records. Active
        // clients re-upsert on every PUT /me/device-token, restoring fresh _ts.
        const int expectedTtlSeconds = 180 * 24 * 60 * 60;
        var registration = DeviceRegistration.Create(
            "auth0|user-ttl",
            "apns-token-ttl",
            DevicePlatform.Ios,
            new DateTimeOffset(2026, 4, 20, 0, 0, 0, TimeSpan.Zero));

        // Act
        var document = DeviceRegistrationDocument.FromDomain(registration);

        // Assert
        await Assert.That(document.Ttl).IsEqualTo(expectedTtlSeconds);
    }
}
