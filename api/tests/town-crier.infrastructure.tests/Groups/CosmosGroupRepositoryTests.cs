using TownCrier.Domain.Geocoding;
using TownCrier.Domain.Groups;
using TownCrier.Infrastructure.Groups;
using TownCrier.Infrastructure.Tests.Cosmos;

namespace TownCrier.Infrastructure.Tests.Groups;

public sealed class CosmosGroupRepositoryTests
{
    [Test]
    public async Task Should_PersistGroup_When_SaveCalled()
    {
        // Arrange
        var client = new FakeCosmosRestClient();
        var repo = new CosmosGroupRepository(client);
        var group = Group.Create("group-1", "Test Group", "user-1", new Coordinates(51.5, -0.1), 500, 100, DateTimeOffset.UtcNow);

        // Act
        await repo.SaveAsync(group, CancellationToken.None);

        // Assert -- GetByIdAsync uses a cross-partition query
        var result = await repo.GetByIdAsync("group-1", CancellationToken.None);
        await Assert.That(result).IsNotNull();
        await Assert.That(result!.Name).IsEqualTo("Test Group");
    }

    [Test]
    public async Task Should_ReturnNull_When_GetByIdForMissingGroup()
    {
        // Arrange
        var client = new FakeCosmosRestClient();
        var repo = new CosmosGroupRepository(client);

        // Act
        var result = await repo.GetByIdAsync("nonexistent", CancellationToken.None);

        // Assert
        await Assert.That(result).IsNull();
    }

    [Test]
    public async Task Should_ReturnGroups_When_GetByUserIdCalled()
    {
        // Arrange
        var client = new FakeCosmosRestClient();
        var repo = new CosmosGroupRepository(client);
        var group = Group.Create("group-1", "Test Group", "user-1", new Coordinates(51.5, -0.1), 500, 100, DateTimeOffset.UtcNow);
        await repo.SaveAsync(group, CancellationToken.None);

        // Act -- fake returns all docs, no SQL filtering for membership
        var result = await repo.GetByUserIdAsync("user-1", CancellationToken.None);

        // Assert
        await Assert.That(result.Count).IsGreaterThanOrEqualTo(1);
    }

    [Test]
    public async Task Should_DeleteGroup_When_DeleteCalled()
    {
        // Arrange
        var client = new FakeCosmosRestClient();
        var repo = new CosmosGroupRepository(client);
        var group = Group.Create("group-1", "Test Group", "user-1", new Coordinates(51.5, -0.1), 500, 100, DateTimeOffset.UtcNow);
        await repo.SaveAsync(group, CancellationToken.None);

        // Act
        await repo.DeleteAsync("group-1", CancellationToken.None);

        // Assert
        var result = await repo.GetByIdAsync("group-1", CancellationToken.None);
        await Assert.That(result).IsNull();
    }
}
