using TownCrier.Application.DeviceRegistrations;
using TownCrier.Domain.DeviceRegistrations;

namespace TownCrier.Application.Tests.DeviceRegistrations;

public sealed class RemoveInvalidDeviceTokenCommandHandlerTests
{
    [Test]
    public async Task Should_RemoveToken_When_TokenExists()
    {
        // Arrange
        var repository = new FakeDeviceRegistrationRepository();
        var registration = DeviceRegistration.Create(
            "auth0|user-123",
            "apns-token-abc123",
            DevicePlatform.Ios,
            DateTimeOffset.UtcNow);
        await repository.SaveAsync(registration, CancellationToken.None);

        var handler = new RemoveInvalidDeviceTokenCommandHandler(repository);
        var command = new RemoveInvalidDeviceTokenCommand("apns-token-abc123");

        // Act
        await handler.HandleAsync(command, CancellationToken.None);

        // Assert
        var result = repository.GetByToken("apns-token-abc123");
        await Assert.That(result).IsNull();
    }

    [Test]
    public async Task Should_NotThrow_When_TokenDoesNotExist()
    {
        // Arrange
        var repository = new FakeDeviceRegistrationRepository();
        var handler = new RemoveInvalidDeviceTokenCommandHandler(repository);
        var command = new RemoveInvalidDeviceTokenCommand("nonexistent-token");

        // Act & Assert — should not throw
        await handler.HandleAsync(command, CancellationToken.None);
        await Assert.That(repository.Count).IsEqualTo(0);
    }

    [Test]
    public async Task Should_OnlyRemoveSpecifiedToken_When_UserHasMultipleDevices()
    {
        // Arrange
        var repository = new FakeDeviceRegistrationRepository();
        var registration1 = DeviceRegistration.Create(
            "auth0|user-123", "token-device1", DevicePlatform.Ios, DateTimeOffset.UtcNow);
        var registration2 = DeviceRegistration.Create(
            "auth0|user-123", "token-device2", DevicePlatform.Ios, DateTimeOffset.UtcNow);
        await repository.SaveAsync(registration1, CancellationToken.None);
        await repository.SaveAsync(registration2, CancellationToken.None);

        var handler = new RemoveInvalidDeviceTokenCommandHandler(repository);

        // Act — remove only device1's token
        await handler.HandleAsync(new RemoveInvalidDeviceTokenCommand("token-device1"), CancellationToken.None);

        // Assert
        await Assert.That(repository.Count).IsEqualTo(1);
        var remaining = repository.GetByToken("token-device2");
        await Assert.That(remaining).IsNotNull();
    }
}
