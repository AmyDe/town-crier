using TownCrier.Application.WatchZones;
using TownCrier.Domain.Geocoding;
using TownCrier.Domain.WatchZones;
using TownCrier.Infrastructure.Tests.Cosmos;
using TownCrier.Infrastructure.WatchZones;

namespace TownCrier.Infrastructure.Tests.WatchZones;

public sealed class CosmosWatchZoneRepositoryTests
{
    [Test]
    public async Task Should_PersistZone_When_SaveCalled()
    {
        // Arrange
        var client = new FakeCosmosRestClient();
        var repo = new CosmosWatchZoneRepository(client);

        var zone = new WatchZone("zone-1", "user-1", "Home", new Coordinates(51.5, -0.1), 500, 100);

        // Act
        await repo.SaveAsync(zone, CancellationToken.None);

        // Assert
        var result = await repo.GetByUserIdAsync("user-1", CancellationToken.None);
        await Assert.That(result.Count).IsEqualTo(1);
        await Assert.That(result.First().Id).IsEqualTo("zone-1");
    }

    [Test]
    public async Task Should_ReturnZones_When_GetByUserIdCalled()
    {
        // Arrange
        var client = new FakeCosmosRestClient();
        var repo = new CosmosWatchZoneRepository(client);

        var zone1 = new WatchZone("zone-1", "user-1", "Home", new Coordinates(51.5, -0.1), 500, 100);
        var zone2 = new WatchZone("zone-2", "user-1", "Work", new Coordinates(51.6, -0.2), 1000, 200);
        await repo.SaveAsync(zone1, CancellationToken.None);
        await repo.SaveAsync(zone2, CancellationToken.None);

        // Act
        var result = await repo.GetByUserIdAsync("user-1", CancellationToken.None);

        // Assert
        await Assert.That(result.Count).IsEqualTo(2);
    }

    [Test]
    public async Task Should_DeleteZone_When_ZoneExists()
    {
        // Arrange
        var client = new FakeCosmosRestClient();
        var repo = new CosmosWatchZoneRepository(client);

        var zone = new WatchZone("zone-1", "user-1", "Home", new Coordinates(51.5, -0.1), 500, 100);
        await repo.SaveAsync(zone, CancellationToken.None);

        // Act
        await repo.DeleteAsync("user-1", "zone-1", CancellationToken.None);

        // Assert
        var result = await repo.GetByUserIdAsync("user-1", CancellationToken.None);
        await Assert.That(result.Count).IsEqualTo(0);
    }

    [Test]
    public async Task Should_ThrowWatchZoneNotFoundException_When_DeletingMissingZone()
    {
        // Arrange
        var client = new FakeCosmosRestClient();
        var repo = new CosmosWatchZoneRepository(client);

        // Act & Assert
        await Assert.ThrowsAsync<WatchZoneNotFoundException>(
            () => repo.DeleteAsync("user-1", "nonexistent", CancellationToken.None));
    }

    [Test]
    public async Task Should_ReturnZones_When_FindZonesContainingCalled()
    {
        // Arrange
        var client = new FakeCosmosRestClient();
        var repo = new CosmosWatchZoneRepository(client);

        var zone = new WatchZone("zone-1", "user-1", "Home", new Coordinates(51.5, -0.1), 500, 100);
        await repo.SaveAsync(zone, CancellationToken.None);

        // Act — fake returns all documents in collection (no SQL parsing)
        var result = await repo.FindZonesContainingAsync(51.5, -0.1, CancellationToken.None);

        // Assert
        await Assert.That(result.Count).IsGreaterThanOrEqualTo(1);
    }

    [Test]
    public async Task Should_ReturnEmptyCollection_When_GetDistinctAuthorityIdsCalledWithNoData()
    {
        // Arrange
        var client = new FakeCosmosRestClient();
        var repo = new CosmosWatchZoneRepository(client);

        // Act — empty store, verifies wiring compiles and runs
        var result = await repo.GetDistinctAuthorityIdsAsync(CancellationToken.None);

        // Assert
        await Assert.That(result.Count).IsEqualTo(0);
    }

    [Test]
    public async Task Should_ReturnEmptyDictionary_When_GetZoneCountsByAuthorityCalledWithNoData()
    {
        // Arrange
        var client = new FakeCosmosRestClient();
        var repo = new CosmosWatchZoneRepository(client);

        // Act — empty store, verifies wiring compiles and runs
        var result = await repo.GetZoneCountsByAuthorityAsync(CancellationToken.None);

        // Assert
        await Assert.That(result.Count).IsEqualTo(0);
    }
}
