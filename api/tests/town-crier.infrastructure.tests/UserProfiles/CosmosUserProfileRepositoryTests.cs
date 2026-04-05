using TownCrier.Domain.UserProfiles;
using TownCrier.Infrastructure.Tests.Cosmos;
using TownCrier.Infrastructure.UserProfiles;

namespace TownCrier.Infrastructure.Tests.UserProfiles;

public sealed class CosmosUserProfileRepositoryTests
{
    [Test]
    public async Task Should_ReturnProfile_When_ProfileExists()
    {
        // Arrange
        var client = new FakeCosmosRestClient();
        var repo = new CosmosUserProfileRepository(client);

        var profile = UserProfile.Register("user-1");
        profile.UpdatePreferences(NotificationPreferences.Default);
        await repo.SaveAsync(profile, CancellationToken.None);

        // Act
        var result = await repo.GetByUserIdAsync("user-1", CancellationToken.None);

        // Assert
        await Assert.That(result).IsNotNull();
        await Assert.That(result!.UserId).IsEqualTo("user-1");
    }

    [Test]
    public async Task Should_ReturnNull_When_ProfileDoesNotExist()
    {
        // Arrange
        var client = new FakeCosmosRestClient();
        var repo = new CosmosUserProfileRepository(client);

        // Act
        var result = await repo.GetByUserIdAsync("nonexistent", CancellationToken.None);

        // Assert
        await Assert.That(result).IsNull();
    }

    [Test]
    public async Task Should_PersistProfile_When_SaveCalled()
    {
        // Arrange
        var client = new FakeCosmosRestClient();
        var repo = new CosmosUserProfileRepository(client);

        var profile = UserProfile.Register("user-2");

        // Act
        await repo.SaveAsync(profile, CancellationToken.None);

        // Assert
        var result = await repo.GetByUserIdAsync("user-2", CancellationToken.None);
        await Assert.That(result).IsNotNull();
        await Assert.That(result!.UserId).IsEqualTo("user-2");
    }

    [Test]
    public async Task Should_RemoveProfile_When_DeleteCalledForExistingProfile()
    {
        // Arrange
        var client = new FakeCosmosRestClient();
        var repo = new CosmosUserProfileRepository(client);

        var profile = UserProfile.Register("user-3");
        await repo.SaveAsync(profile, CancellationToken.None);

        // Act
        await repo.DeleteAsync("user-3", CancellationToken.None);

        // Assert
        var result = await repo.GetByUserIdAsync("user-3", CancellationToken.None);
        await Assert.That(result).IsNull();
    }

    [Test]
    public async Task Should_NotThrow_When_DeleteCalledForMissingProfile()
    {
        // Arrange
        var client = new FakeCosmosRestClient();
        var repo = new CosmosUserProfileRepository(client);

        // Act & Assert — should not throw
        await repo.DeleteAsync("nonexistent", CancellationToken.None);
    }

    [Test]
    public async Task Should_ReturnProfiles_When_GetAllByTierCalled()
    {
        // Arrange
        var client = new FakeCosmosRestClient();
        var repo = new CosmosUserProfileRepository(client);

        var profile1 = UserProfile.Register("user-a");
        var profile2 = UserProfile.Register("user-b");
        await repo.SaveAsync(profile1, CancellationToken.None);
        await repo.SaveAsync(profile2, CancellationToken.None);

        // Act — both default to Free tier
        var result = await repo.GetAllByTierAsync(SubscriptionTier.Free, CancellationToken.None);

        // Assert
        await Assert.That(result.Count).IsGreaterThanOrEqualTo(2);
    }
}
