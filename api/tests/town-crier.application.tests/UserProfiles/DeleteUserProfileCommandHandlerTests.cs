using TownCrier.Application.Tests.Admin;
using TownCrier.Application.UserProfiles;
using TownCrier.Domain.UserProfiles;

namespace TownCrier.Application.Tests.UserProfiles;

public sealed class DeleteUserProfileCommandHandlerTests
{
    [Test]
    public async Task Should_DeleteProfile_When_ProfileExists()
    {
        // Arrange
        var repository = new FakeUserProfileRepository();
        var auth0Client = new FakeAuth0ManagementClient();
        var profile = UserProfile.Register("auth0|user1");
        await repository.SaveAsync(profile, CancellationToken.None);
        var handler = new DeleteUserProfileCommandHandler(repository, auth0Client);

        // Act
        await handler.HandleAsync(
            new DeleteUserProfileCommand("auth0|user1"), CancellationToken.None);

        // Assert
        await Assert.That(repository.GetByUserId("auth0|user1")).IsNull();
    }

    [Test]
    public async Task Should_ThrowNotFound_When_ProfileDoesNotExist()
    {
        // Arrange
        var repository = new FakeUserProfileRepository();
        var auth0Client = new FakeAuth0ManagementClient();
        var handler = new DeleteUserProfileCommandHandler(repository, auth0Client);

        // Act & Assert
        await Assert.ThrowsAsync<UserProfileNotFoundException>(
            () => handler.HandleAsync(
                new DeleteUserProfileCommand("auth0|nonexistent"), CancellationToken.None));
    }

    [Test]
    public async Task Should_NotAffectOtherProfiles_When_DeletingOne()
    {
        // Arrange
        var repository = new FakeUserProfileRepository();
        var auth0Client = new FakeAuth0ManagementClient();
        await repository.SaveAsync(UserProfile.Register("auth0|user1"), CancellationToken.None);
        await repository.SaveAsync(UserProfile.Register("auth0|user2"), CancellationToken.None);
        var handler = new DeleteUserProfileCommandHandler(repository, auth0Client);

        // Act
        await handler.HandleAsync(
            new DeleteUserProfileCommand("auth0|user1"), CancellationToken.None);

        // Assert
        await Assert.That(repository.GetByUserId("auth0|user1")).IsNull();
        await Assert.That(repository.GetByUserId("auth0|user2")).IsNotNull();
        await Assert.That(repository.Count).IsEqualTo(1);
    }

    [Test]
    public async Task Should_DeleteAuth0User_When_ProfileExists()
    {
        // Arrange
        var repository = new FakeUserProfileRepository();
        var auth0Client = new FakeAuth0ManagementClient();
        var profile = UserProfile.Register("auth0|user1");
        await repository.SaveAsync(profile, CancellationToken.None);
        var handler = new DeleteUserProfileCommandHandler(repository, auth0Client);

        // Act
        await handler.HandleAsync(
            new DeleteUserProfileCommand("auth0|user1"), CancellationToken.None);

        // Assert
        await Assert.That(auth0Client.Deletions).HasCount().EqualTo(1);
        await Assert.That(auth0Client.Deletions[0]).IsEqualTo("auth0|user1");
    }

    [Test]
    public async Task Should_NotCallAuth0_When_ProfileDoesNotExist()
    {
        // Arrange
        var repository = new FakeUserProfileRepository();
        var auth0Client = new FakeAuth0ManagementClient();
        var handler = new DeleteUserProfileCommandHandler(repository, auth0Client);

        // Act
        try
        {
            await handler.HandleAsync(
                new DeleteUserProfileCommand("auth0|nonexistent"), CancellationToken.None);
        }
        catch (UserProfileNotFoundException)
        {
            // Expected — profile was missing.
        }

        // Assert
        await Assert.That(auth0Client.Deletions).IsEmpty();
    }
}
