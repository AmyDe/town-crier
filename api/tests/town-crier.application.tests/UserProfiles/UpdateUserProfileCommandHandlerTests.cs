using TownCrier.Application.UserProfiles;
using TownCrier.Domain.UserProfiles;

namespace TownCrier.Application.Tests.UserProfiles;

public sealed class UpdateUserProfileCommandHandlerTests
{
    [Test]
    public async Task Should_UpdatePostcode_When_ProfileExists()
    {
        // Arrange
        var repository = new FakeUserProfileRepository();
        var profile = UserProfile.Register("auth0|user-789");
        await repository.SaveAsync(profile, CancellationToken.None);

        var handler = new UpdateUserProfileCommandHandler(repository);
        var command = new UpdateUserProfileCommand("auth0|user-789", "SW1A 1AA", true);

        // Act
        var result = await handler.HandleAsync(command, CancellationToken.None);

        // Assert
        await Assert.That(result.Postcode).IsEqualTo("SW1A 1AA");
        await Assert.That(result.PushEnabled).IsTrue();
    }

    [Test]
    public async Task Should_UpdateNotificationPreferences_When_ProfileExists()
    {
        // Arrange
        var repository = new FakeUserProfileRepository();
        var profile = UserProfile.Register("auth0|user-789");
        await repository.SaveAsync(profile, CancellationToken.None);

        var handler = new UpdateUserProfileCommandHandler(repository);
        var command = new UpdateUserProfileCommand("auth0|user-789", null, false);

        // Act
        var result = await handler.HandleAsync(command, CancellationToken.None);

        // Assert
        await Assert.That(result.PushEnabled).IsFalse();
        await Assert.That(result.Postcode).IsNull();
    }

    [Test]
    public async Task Should_PersistChanges_When_PreferencesUpdated()
    {
        // Arrange
        var repository = new FakeUserProfileRepository();
        var profile = UserProfile.Register("auth0|user-789");
        await repository.SaveAsync(profile, CancellationToken.None);

        var handler = new UpdateUserProfileCommandHandler(repository);
        var command = new UpdateUserProfileCommand("auth0|user-789", "EC1A 1BB", false);

        // Act
        await handler.HandleAsync(command, CancellationToken.None);

        // Assert
        var saved = repository.GetByUserId("auth0|user-789");
        await Assert.That(saved!.Postcode).IsEqualTo("EC1A 1BB");
        await Assert.That(saved.NotificationPreferences.PushEnabled).IsFalse();
    }

    [Test]
    public async Task Should_ThrowUserProfileNotFoundException_When_ProfileDoesNotExist()
    {
        // Arrange
        var repository = new FakeUserProfileRepository();
        var handler = new UpdateUserProfileCommandHandler(repository);
        var command = new UpdateUserProfileCommand("auth0|nonexistent", "SW1A 1AA", true);

        // Act & Assert
        await Assert.ThrowsAsync<UserProfileNotFoundException>(
            () => handler.HandleAsync(command, CancellationToken.None));
    }

    [Test]
    public async Task Should_PreserveTier_When_PreferencesUpdated()
    {
        // Arrange
        var repository = new FakeUserProfileRepository();
        var profile = UserProfile.Register("auth0|user-789");
        await repository.SaveAsync(profile, CancellationToken.None);

        var handler = new UpdateUserProfileCommandHandler(repository);
        var command = new UpdateUserProfileCommand("auth0|user-789", "SW1A 1AA", false);

        // Act
        var result = await handler.HandleAsync(command, CancellationToken.None);

        // Assert — tier should remain Free (not modifiable via preferences update)
        await Assert.That(result.Tier).IsEqualTo(SubscriptionTier.Free);
    }
}
