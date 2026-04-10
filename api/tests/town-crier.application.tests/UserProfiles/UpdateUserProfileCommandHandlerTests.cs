using TownCrier.Application.UserProfiles;
using TownCrier.Domain.UserProfiles;

namespace TownCrier.Application.Tests.UserProfiles;

public sealed class UpdateUserProfileCommandHandlerTests
{
    [Test]
    public async Task Should_UpdatePushEnabled_When_ProfileExists()
    {
        // Arrange
        var repository = new FakeUserProfileRepository();
        var profile = UserProfile.Register("auth0|user-789");
        await repository.SaveAsync(profile, CancellationToken.None);

        var handler = new UpdateUserProfileCommandHandler(repository);
        var command = new UpdateUserProfileCommand("auth0|user-789", true);

        // Act
        var result = await handler.HandleAsync(command, CancellationToken.None);

        // Assert
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
        var command = new UpdateUserProfileCommand("auth0|user-789", false);

        // Act
        var result = await handler.HandleAsync(command, CancellationToken.None);

        // Assert
        await Assert.That(result.PushEnabled).IsFalse();
    }

    [Test]
    public async Task Should_PersistChanges_When_PreferencesUpdated()
    {
        // Arrange
        var repository = new FakeUserProfileRepository();
        var profile = UserProfile.Register("auth0|user-789");
        await repository.SaveAsync(profile, CancellationToken.None);

        var handler = new UpdateUserProfileCommandHandler(repository);
        var command = new UpdateUserProfileCommand("auth0|user-789", false);

        // Act
        await handler.HandleAsync(command, CancellationToken.None);

        // Assert
        var saved = repository.GetByUserId("auth0|user-789");
        await Assert.That(saved!.NotificationPreferences.PushEnabled).IsFalse();
    }

    [Test]
    public async Task Should_ThrowUserProfileNotFoundException_When_ProfileDoesNotExist()
    {
        // Arrange
        var repository = new FakeUserProfileRepository();
        var handler = new UpdateUserProfileCommandHandler(repository);
        var command = new UpdateUserProfileCommand("auth0|nonexistent", true);

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
        var command = new UpdateUserProfileCommand("auth0|user-789", false);

        // Act
        var result = await handler.HandleAsync(command, CancellationToken.None);

        // Assert — tier should remain Free (not modifiable via preferences update)
        await Assert.That(result.Tier).IsEqualTo(SubscriptionTier.Free);
    }

    [Test]
    public async Task Should_UpdateEmailDigestEnabled_When_SetToFalse()
    {
        // Arrange
        var repository = new FakeUserProfileRepository();
        var profile = UserProfile.Register("auth0|user-789");
        await repository.SaveAsync(profile, CancellationToken.None);

        var handler = new UpdateUserProfileCommandHandler(repository);
        var command = new UpdateUserProfileCommand("auth0|user-789", true, EmailDigestEnabled: false);

        // Act
        var result = await handler.HandleAsync(command, CancellationToken.None);

        // Assert
        await Assert.That(result.EmailDigestEnabled).IsFalse();
        var saved = repository.GetByUserId("auth0|user-789");
        await Assert.That(saved!.NotificationPreferences.EmailDigestEnabled).IsFalse();
    }

    [Test]
    public async Task Should_UpdateDigestDay_When_SetToFriday()
    {
        // Arrange
        var repository = new FakeUserProfileRepository();
        var profile = UserProfile.Register("auth0|user-789");
        await repository.SaveAsync(profile, CancellationToken.None);

        var handler = new UpdateUserProfileCommandHandler(repository);
        var command = new UpdateUserProfileCommand("auth0|user-789", true, DigestDay: DayOfWeek.Friday);

        // Act
        var result = await handler.HandleAsync(command, CancellationToken.None);

        // Assert
        await Assert.That(result.DigestDay).IsEqualTo(DayOfWeek.Friday);
        var saved = repository.GetByUserId("auth0|user-789");
        await Assert.That(saved!.NotificationPreferences.DigestDay).IsEqualTo(DayOfWeek.Friday);
    }
}
