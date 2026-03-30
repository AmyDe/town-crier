using TownCrier.Domain.SavedApplications;
using TownCrier.Infrastructure.SavedApplications;
using TownCrier.Infrastructure.Tests.Cosmos;

namespace TownCrier.Infrastructure.Tests.SavedApplications;

public sealed class CosmosSavedApplicationRepositoryTests
{
    [Test]
    public async Task Should_PersistSavedApplication_When_SaveCalled()
    {
        // Arrange
        var client = new FakeCosmosRestClient();
        var repo = new CosmosSavedApplicationRepository(client);
        var saved = SavedApplication.Create("user-1", "app-uid-1", DateTimeOffset.UtcNow);

        // Act
        await repo.SaveAsync(saved, CancellationToken.None);

        // Assert
        var result = await repo.GetByUserIdAsync("user-1", CancellationToken.None);
        await Assert.That(result.Count).IsEqualTo(1);
        await Assert.That(result[0].ApplicationUid).IsEqualTo("app-uid-1");
    }

    [Test]
    public async Task Should_DeleteSavedApplication_When_DeleteCalled()
    {
        // Arrange
        var client = new FakeCosmosRestClient();
        var repo = new CosmosSavedApplicationRepository(client);
        var saved = SavedApplication.Create("user-1", "app-uid-1", DateTimeOffset.UtcNow);
        await repo.SaveAsync(saved, CancellationToken.None);

        // Act
        await repo.DeleteAsync("user-1", "app-uid-1", CancellationToken.None);

        // Assert
        var result = await repo.GetByUserIdAsync("user-1", CancellationToken.None);
        await Assert.That(result.Count).IsEqualTo(0);
    }

    [Test]
    public async Task Should_NotThrow_When_DeleteCalledForMissingApplication()
    {
        // Arrange
        var client = new FakeCosmosRestClient();
        var repo = new CosmosSavedApplicationRepository(client);

        // Act & Assert — should not throw (idempotent delete)
        await repo.DeleteAsync("user-1", "nonexistent", CancellationToken.None);
    }

    [Test]
    public async Task Should_ReturnTrue_When_ExistsForExistingApplication()
    {
        // Arrange
        var client = new FakeCosmosRestClient();
        var repo = new CosmosSavedApplicationRepository(client);
        var saved = SavedApplication.Create("user-1", "app-uid-1", DateTimeOffset.UtcNow);
        await repo.SaveAsync(saved, CancellationToken.None);

        // Act
        var result = await repo.ExistsAsync("user-1", "app-uid-1", CancellationToken.None);

        // Assert
        await Assert.That(result).IsTrue();
    }

    [Test]
    public async Task Should_ReturnFalse_When_ExistsForMissingApplication()
    {
        // Arrange
        var client = new FakeCosmosRestClient();
        var repo = new CosmosSavedApplicationRepository(client);

        // Act
        var result = await repo.ExistsAsync("user-1", "nonexistent", CancellationToken.None);

        // Assert
        await Assert.That(result).IsFalse();
    }

    [Test]
    public async Task Should_ReturnEmptyList_When_GetUserIdsByApplicationUidWithNoData()
    {
        // Arrange
        var client = new FakeCosmosRestClient();
        var repo = new CosmosSavedApplicationRepository(client);

        // Act -- empty store, verifies wiring
        var result = await repo.GetUserIdsByApplicationUidAsync("nonexistent", CancellationToken.None);

        // Assert
        await Assert.That(result.Count).IsEqualTo(0);
    }
}
