using TownCrier.Domain.PlanningApplications;
using TownCrier.Infrastructure.PlanningApplications;
using TownCrier.Infrastructure.Tests.Cosmos;

namespace TownCrier.Infrastructure.Tests.PlanningApplications;

public sealed class CosmosPlanningApplicationRepositoryTests
{
    [Test]
    public async Task Should_PersistApplication_When_UpsertCalled()
    {
        // Arrange
        var client = new FakeCosmosRestClient();
        var repo = new CosmosPlanningApplicationRepository(client);
        var app = CreateTestApplication();

        // Act
        await repo.UpsertAsync(app, CancellationToken.None);

        // Assert
        var result = await repo.GetByUidAsync("uid-1", CancellationToken.None);
        await Assert.That(result).IsNotNull();
        await Assert.That(result!.Uid).IsEqualTo("uid-1");
    }

    [Test]
    public async Task Should_ReturnNull_When_GetByUidForMissingApplication()
    {
        // Arrange
        var client = new FakeCosmosRestClient();
        var repo = new CosmosPlanningApplicationRepository(client);

        // Act
        var result = await repo.GetByUidAsync("nonexistent", CancellationToken.None);

        // Assert
        await Assert.That(result).IsNull();
    }

    [Test]
    public async Task Should_ReturnApplications_When_GetByAuthorityIdCalled()
    {
        // Arrange
        var client = new FakeCosmosRestClient();
        var repo = new CosmosPlanningApplicationRepository(client);
        var app = CreateTestApplication();
        await repo.UpsertAsync(app, CancellationToken.None);

        // Act
        var result = await repo.GetByAuthorityIdAsync(100, CancellationToken.None);

        // Assert
        await Assert.That(result.Count).IsGreaterThanOrEqualTo(1);
    }

    [Test]
    public async Task Should_ReturnApplications_When_FindNearbyCalled()
    {
        // Arrange
        var client = new FakeCosmosRestClient();
        var repo = new CosmosPlanningApplicationRepository(client);
        var app = CreateTestApplication();
        await repo.UpsertAsync(app, CancellationToken.None);

        // Act -- fake returns all in partition, not geo-filtered
        var result = await repo.FindNearbyAsync("100", 51.5, -0.1, 500, CancellationToken.None);

        // Assert
        await Assert.That(result.Count).IsGreaterThanOrEqualTo(1);
    }

#pragma warning disable S1075 // Test data URIs are intentionally hardcoded
    private static PlanningApplication CreateTestApplication(string name = "APP/001", string uid = "uid-1", int areaId = 100)
    {
        return new PlanningApplication(
            name: name,
            uid: uid,
            areaName: "Test Area",
            areaId: areaId,
            address: "123 Main St",
            postcode: "SW1A 1AA",
            description: "Test application",
            appType: "Full",
            appState: "Pending",
            appSize: null,
            startDate: new DateOnly(2026, 1, 1),
            decidedDate: null,
            consultedDate: null,
            longitude: -0.1,
            latitude: 51.5,
            url: "https://example.com",
            link: "https://example.com/link",
            lastDifferent: DateTimeOffset.UtcNow);
    }
#pragma warning restore S1075
}
