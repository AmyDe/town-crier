using TownCrier.Application.DeviceRegistrations;
using TownCrier.Domain.DeviceRegistrations;

namespace TownCrier.Application.Tests.DeviceRegistrations;

public sealed class RegisterDeviceTokenCommandHandlerTests
{
    [Test]
    public async Task Should_StoreDeviceToken_When_NewRegistration()
    {
        // Arrange
        var repository = new FakeDeviceRegistrationRepository();
        var timeProvider = new FakeTimeProvider(new DateTimeOffset(2026, 3, 17, 10, 0, 0, TimeSpan.Zero));
        var handler = new RegisterDeviceTokenCommandHandler(repository, timeProvider);

        var command = new RegisterDeviceTokenCommand(
            "auth0|user-123",
            "apns-token-abc123",
            DevicePlatform.Ios);

        // Act
        await handler.HandleAsync(command, CancellationToken.None);

        // Assert
        var saved = repository.GetByToken("apns-token-abc123");
        await Assert.That(saved).IsNotNull();
        await Assert.That(saved!.UserId).IsEqualTo("auth0|user-123");
        await Assert.That(saved.Token).IsEqualTo("apns-token-abc123");
        await Assert.That(saved.Platform).IsEqualTo(DevicePlatform.Ios);
        await Assert.That(saved.RegisteredAt).IsEqualTo(timeProvider.GetUtcNow());
    }

    [Test]
    public async Task Should_RefreshTimestamp_When_TokenAlreadyRegistered()
    {
        // Arrange
        var repository = new FakeDeviceRegistrationRepository();
        var timeProvider = new FakeTimeProvider(new DateTimeOffset(2026, 3, 17, 10, 0, 0, TimeSpan.Zero));
        var handler = new RegisterDeviceTokenCommandHandler(repository, timeProvider);

        var command = new RegisterDeviceTokenCommand(
            "auth0|user-123",
            "apns-token-abc123",
            DevicePlatform.Ios);

        await handler.HandleAsync(command, CancellationToken.None);

        // Advance time
        timeProvider.Advance(TimeSpan.FromDays(1));

        // Act — re-register same token
        await handler.HandleAsync(command, CancellationToken.None);

        // Assert — still one registration, updated timestamp
        await Assert.That(repository.Count).IsEqualTo(1);
        var saved = repository.GetByToken("apns-token-abc123");
        await Assert.That(saved!.RegisteredAt).IsEqualTo(timeProvider.GetUtcNow());
    }

    [Test]
    public async Task Should_StoreMultipleTokens_When_UserHasMultipleDevices()
    {
        // Arrange
        var repository = new FakeDeviceRegistrationRepository();
        var timeProvider = new FakeTimeProvider(new DateTimeOffset(2026, 3, 17, 10, 0, 0, TimeSpan.Zero));
        var handler = new RegisterDeviceTokenCommandHandler(repository, timeProvider);

        // Act — register two different tokens for same user
        await handler.HandleAsync(
            new RegisterDeviceTokenCommand("auth0|user-123", "apns-token-device1", DevicePlatform.Ios),
            CancellationToken.None);
        await handler.HandleAsync(
            new RegisterDeviceTokenCommand("auth0|user-123", "apns-token-device2", DevicePlatform.Ios),
            CancellationToken.None);

        // Assert
        var userTokens = await repository.GetByUserIdAsync("auth0|user-123", CancellationToken.None);
        await Assert.That(userTokens.Count).IsEqualTo(2);
    }
}
