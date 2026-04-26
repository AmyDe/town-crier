using TownCrier.Domain.DecisionAlerts;
using TownCrier.Infrastructure.DecisionAlerts;
using TownCrier.Infrastructure.Tests.Cosmos;

namespace TownCrier.Infrastructure.Tests.DecisionAlerts;

public sealed class CosmosDecisionAlertRepositoryTests
{
    [Test]
    public async Task Should_ReturnAlert_When_AlertExists()
    {
        // Arrange
        var client = new FakeCosmosRestClient();
        var repo = new CosmosDecisionAlertRepository(client);

        var alert = DecisionAlert.Create("user-1", "app-uid-1", "APP/001", "123 Main St", "Permitted", DateTimeOffset.UtcNow);
        await repo.SaveAsync(alert, CancellationToken.None);

        // Act
        var result = await repo.GetByUserAndApplicationAsync("user-1", "app-uid-1", CancellationToken.None);

        // Assert
        await Assert.That(result).IsNotNull();
        await Assert.That(result!.UserId).IsEqualTo("user-1");
        await Assert.That(result.ApplicationUid).IsEqualTo("app-uid-1");
    }

    [Test]
    public async Task Should_ReturnNull_When_AlertDoesNotExist()
    {
        // Arrange
        var client = new FakeCosmosRestClient();
        var repo = new CosmosDecisionAlertRepository(client);

        // Act
        var result = await repo.GetByUserAndApplicationAsync("user-1", "nonexistent", CancellationToken.None);

        // Assert
        await Assert.That(result).IsNull();
    }

    [Test]
    public async Task Should_PersistAlert_When_SaveCalled()
    {
        // Arrange
        var client = new FakeCosmosRestClient();
        var repo = new CosmosDecisionAlertRepository(client);

        var alert = DecisionAlert.Create("user-2", "app-uid-2", "APP/002", "456 High St", "Rejected", DateTimeOffset.UtcNow);

        // Act
        await repo.SaveAsync(alert, CancellationToken.None);

        // Assert
        var result = await repo.GetByUserAndApplicationAsync("user-2", "app-uid-2", CancellationToken.None);
        await Assert.That(result).IsNotNull();
        await Assert.That(result!.Decision).IsEqualTo("Rejected");
    }
}
