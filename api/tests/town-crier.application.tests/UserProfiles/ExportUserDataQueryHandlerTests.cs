using TownCrier.Application.UserProfiles;
using TownCrier.Domain.UserProfiles;

namespace TownCrier.Application.Tests.UserProfiles;

public sealed class ExportUserDataQueryHandlerTests
{
    [Test]
    public async Task Should_ReturnUserData_When_ProfileExists()
    {
        // Arrange
        var repository = new FakeUserProfileRepository();
        var profile = UserProfile.Register("auth0|user1");
        profile.UpdatePreferences("SW1A 1AA", new NotificationPreferences(PushEnabled: true));
        await repository.SaveAsync(profile, CancellationToken.None);
        var handler = new ExportUserDataQueryHandler(repository);

        // Act
        var result = await handler.HandleAsync(
            new ExportUserDataQuery("auth0|user1"), CancellationToken.None);

        // Assert
        await Assert.That(result).IsNotNull();
        await Assert.That(result!.UserId).IsEqualTo("auth0|user1");
        await Assert.That(result.Postcode).IsEqualTo("SW1A 1AA");
        await Assert.That(result.PushEnabled).IsTrue();
        await Assert.That(result.Tier).IsEqualTo(SubscriptionTier.Free);
    }

    [Test]
    public async Task Should_ReturnNull_When_ProfileDoesNotExist()
    {
        // Arrange
        var repository = new FakeUserProfileRepository();
        var handler = new ExportUserDataQueryHandler(repository);

        // Act
        var result = await handler.HandleAsync(
            new ExportUserDataQuery("auth0|nonexistent"), CancellationToken.None);

        // Assert
        await Assert.That(result).IsNull();
    }
}
