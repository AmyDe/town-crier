using TownCrier.Application.UserProfiles;
using TownCrier.Domain.UserProfiles;

namespace TownCrier.Application.Tests.UserProfiles;

public sealed class GetUserProfileQueryHandlerTests
{
    [Test]
    public async Task Should_ReturnNull_When_ProfileDoesNotExist()
    {
        // Arrange
        var repository = new FakeUserProfileRepository();
        var handler = new GetUserProfileQueryHandler(repository);
        var query = new GetUserProfileQuery("auth0|nonexistent");

        // Act
        var result = await handler.HandleAsync(query, CancellationToken.None);

        // Assert
        await Assert.That(result).IsNull();
    }

    [Test]
    public async Task Should_ReturnProfile_When_ProfileExists()
    {
        // Arrange
        var repository = new FakeUserProfileRepository();
        var profile = UserProfile.Register("auth0|user-456");
        await repository.SaveAsync(profile, CancellationToken.None);

        var handler = new GetUserProfileQueryHandler(repository);
        var query = new GetUserProfileQuery("auth0|user-456");

        // Act
        var result = await handler.HandleAsync(query, CancellationToken.None);

        // Assert
        await Assert.That(result).IsNotNull();
        await Assert.That(result!.UserId).IsEqualTo("auth0|user-456");
        await Assert.That(result.Tier).IsEqualTo(SubscriptionTier.Free);
        await Assert.That(result.PushEnabled).IsTrue();
    }

    [Test]
    public async Task Should_ReturnDefaultEmailPreferences_When_ProfileExists()
    {
        // Arrange
        var repository = new FakeUserProfileRepository();
        var profile = UserProfile.Register("auth0|email-user");
        await repository.SaveAsync(profile, CancellationToken.None);

        var handler = new GetUserProfileQueryHandler(repository);
        var query = new GetUserProfileQuery("auth0|email-user");

        // Act
        var result = await handler.HandleAsync(query, CancellationToken.None);

        // Assert
        await Assert.That(result).IsNotNull();
        await Assert.That(result!.EmailDigestEnabled).IsTrue();
        await Assert.That(result.DigestDay).IsEqualTo(DayOfWeek.Monday);
    }
}
