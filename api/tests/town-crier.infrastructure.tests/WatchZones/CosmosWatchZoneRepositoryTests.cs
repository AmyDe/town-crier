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

        var zone = new WatchZone("zone-1", "user-1", "Home", new Coordinates(51.5, -0.1), 500, 100, DateTimeOffset.MinValue);

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

        var zone1 = new WatchZone("zone-1", "user-1", "Home", new Coordinates(51.5, -0.1), 500, 100, DateTimeOffset.MinValue);
        var zone2 = new WatchZone("zone-2", "user-1", "Work", new Coordinates(51.6, -0.2), 1000, 200, DateTimeOffset.MinValue);
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

        var zone = new WatchZone("zone-1", "user-1", "Home", new Coordinates(51.5, -0.1), 500, 100, DateTimeOffset.MinValue);
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

        var zone = new WatchZone("zone-1", "user-1", "Home", new Coordinates(51.5, -0.1), 500, 100, DateTimeOffset.MinValue);
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
    public async Task Should_DeduplicateAuthorityIds_When_QueryReturnsDuplicatesFromPartitionFanOut()
    {
        // Arrange — simulate partition fan-out returning duplicate DISTINCT results
        var client = new FakeCosmosRestClient();
        client.SetQueryResults("SELECT DISTINCT VALUE c.authorityId", new List<int> { 1, 2, 2, 3, 3, 3 });
        var repo = new CosmosWatchZoneRepository(client);

        // Act
        var result = await repo.GetDistinctAuthorityIdsAsync(CancellationToken.None);

        // Assert — duplicates from overlapping partition ranges must be removed
        await Assert.That(result.Count).IsEqualTo(3);
        await Assert.That(result).Contains(1);
        await Assert.That(result).Contains(2);
        await Assert.That(result).Contains(3);
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

    [Test]
    public async Task Should_ReaggregateZoneCounts_When_QueryReturnsPartialAggregatesFromPartitionFanOut()
    {
        // Arrange — simulate partition fan-out returning partial GROUP BY aggregates.
        // Authority 1 appears in two partition ranges (counts 3 + 2 = 5),
        // Authority 2 appears once (count 4).
        var client = new FakeCosmosRestClient();
        client.SetQueryResults(
            "SELECT c.authorityId, COUNT(1) AS zoneCount",
            new List<AuthorityZoneCountResult>
            {
                new() { AuthorityId = 1, ZoneCount = 3 },
                new() { AuthorityId = 2, ZoneCount = 4 },
                new() { AuthorityId = 1, ZoneCount = 2 },
            });
        var repo = new CosmosWatchZoneRepository(client);

        // Act
        var result = await repo.GetZoneCountsByAuthorityAsync(CancellationToken.None);

        // Assert — partial aggregates for same authority must be summed
        await Assert.That(result.Count).IsEqualTo(2);
        await Assert.That(result[1]).IsEqualTo(5);
        await Assert.That(result[2]).IsEqualTo(4);
    }
}
