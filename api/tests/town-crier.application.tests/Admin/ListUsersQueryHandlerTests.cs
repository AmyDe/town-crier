using TownCrier.Application.Admin;
using TownCrier.Application.Tests.UserProfiles;
using TownCrier.Domain.UserProfiles;

namespace TownCrier.Application.Tests.Admin;

public sealed class ListUsersQueryHandlerTests
{
    [Test]
    public async Task Should_ReturnMappedItems_When_ProfilesExist()
    {
        // Arrange
        var repository = new FakeUserProfileRepository();
        await repository.SaveAsync(
            UserProfile.Register("auth0|user-1", "alice@example.com"), CancellationToken.None);
        await repository.SaveAsync(
            UserProfile.Register("auth0|user-2", "bob@example.com"), CancellationToken.None);

        var handler = new ListUsersQueryHandler(repository);
        var query = new ListUsersQuery(null, 20, null);

        // Act
        var result = await handler.HandleAsync(query, CancellationToken.None);

        // Assert
        await Assert.That(result.Items).HasCount().EqualTo(2);
        await Assert.That(result.Items[0].UserId).IsEqualTo("auth0|user-1");
        await Assert.That(result.Items[0].Email).IsEqualTo("alice@example.com");
        await Assert.That(result.Items[0].Tier).IsEqualTo(SubscriptionTier.Free);
    }

    [Test]
    public async Task Should_FilterByEmail_When_SearchTermProvided()
    {
        // Arrange
        var repository = new FakeUserProfileRepository();
        await repository.SaveAsync(
            UserProfile.Register("auth0|user-1", "alice@gmail.com"), CancellationToken.None);
        await repository.SaveAsync(
            UserProfile.Register("auth0|user-2", "bob@outlook.com"), CancellationToken.None);

        var handler = new ListUsersQueryHandler(repository);
        var query = new ListUsersQuery("gmail", 20, null);

        // Act
        var result = await handler.HandleAsync(query, CancellationToken.None);

        // Assert
        await Assert.That(result.Items).HasCount().EqualTo(1);
        await Assert.That(result.Items[0].Email).IsEqualTo("alice@gmail.com");
    }

    [Test]
    public async Task Should_ReturnEmptyList_When_NoProfilesExist()
    {
        // Arrange
        var repository = new FakeUserProfileRepository();
        var handler = new ListUsersQueryHandler(repository);
        var query = new ListUsersQuery(null, 20, null);

        // Act
        var result = await handler.HandleAsync(query, CancellationToken.None);

        // Assert
        await Assert.That(result.Items).HasCount().EqualTo(0);
        await Assert.That(result.ContinuationToken).IsNull();
    }

    [Test]
    public async Task Should_MapTierCorrectly_When_UserHasProSubscription()
    {
        // Arrange
        var repository = new FakeUserProfileRepository();
        var profile = UserProfile.Register("auth0|user-1", "alice@example.com");
        profile.ActivateSubscription(SubscriptionTier.Pro, DateTimeOffset.UtcNow.AddYears(1));
        await repository.SaveAsync(profile, CancellationToken.None);

        var handler = new ListUsersQueryHandler(repository);
        var query = new ListUsersQuery(null, 20, null);

        // Act
        var result = await handler.HandleAsync(query, CancellationToken.None);

        // Assert
        await Assert.That(result.Items[0].Tier).IsEqualTo(SubscriptionTier.Pro);
    }
}
