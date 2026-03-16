using TownCrier.Application.UserProfiles;
using TownCrier.Domain.UserProfiles;

namespace TownCrier.Application.Tests.UserProfiles;

public sealed class CreateUserProfileCommandHandlerTests
{
    [Test]
    public async Task Should_CreateProfileWithFreeTier_When_NewUser()
    {
        // Arrange
        var repository = new FakeUserProfileRepository();
        var handler = new CreateUserProfileCommandHandler(repository);
        var command = new CreateUserProfileCommand("auth0|user-123");

        // Act
        var result = await handler.HandleAsync(command, CancellationToken.None);

        // Assert
        await Assert.That(result.UserId).IsEqualTo("auth0|user-123");
        await Assert.That(result.Tier).IsEqualTo(SubscriptionTier.Free);
        await Assert.That(result.PushEnabled).IsTrue();
        await Assert.That(result.Postcode).IsNull();
    }

    [Test]
    public async Task Should_PersistProfile_When_NewUser()
    {
        // Arrange
        var repository = new FakeUserProfileRepository();
        var handler = new CreateUserProfileCommandHandler(repository);
        var command = new CreateUserProfileCommand("auth0|user-123");

        // Act
        await handler.HandleAsync(command, CancellationToken.None);

        // Assert
        var saved = repository.GetByUserId("auth0|user-123");
        await Assert.That(saved).IsNotNull();
        await Assert.That(saved!.Tier).IsEqualTo(SubscriptionTier.Free);
    }

    [Test]
    public async Task Should_ReturnExistingProfile_When_UserAlreadyExists()
    {
        // Arrange
        var repository = new FakeUserProfileRepository();
        var handler = new CreateUserProfileCommandHandler(repository);
        var command = new CreateUserProfileCommand("auth0|user-123");

        // Create first
        await handler.HandleAsync(command, CancellationToken.None);

        // Act — create again
        var result = await handler.HandleAsync(command, CancellationToken.None);

        // Assert — still one profile, returns existing
        await Assert.That(repository.Count).IsEqualTo(1);
        await Assert.That(result.UserId).IsEqualTo("auth0|user-123");
    }
}
