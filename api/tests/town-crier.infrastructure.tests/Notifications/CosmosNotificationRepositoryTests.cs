using TownCrier.Domain.Notifications;
using TownCrier.Infrastructure.Notifications;
using TownCrier.Infrastructure.Tests.Cosmos;

namespace TownCrier.Infrastructure.Tests.Notifications;

public sealed class CosmosNotificationRepositoryTests
{
    [Test]
    public async Task Should_PersistNotification_When_SaveCalled()
    {
        // Arrange
        var client = new FakeCosmosRestClient();
        var repo = new CosmosNotificationRepository(client);
        var notification = new NotificationBuilder().Build();

        // Act
        await repo.SaveAsync(notification, CancellationToken.None);

        // Assert
        var result = await repo.GetByUserAndApplicationAsync(
            "user-1", "test-uid-001", NotificationEventType.NewApplication, CancellationToken.None);
        await Assert.That(result).IsNotNull();
    }

    [Test]
    public async Task Should_ReturnNull_When_GetByUserAndApplicationForMissing()
    {
        // Arrange
        var client = new FakeCosmosRestClient();
        var repo = new CosmosNotificationRepository(client);

        // Act
        var result = await repo.GetByUserAndApplicationAsync(
            "user-1", "nonexistent", NotificationEventType.NewApplication, CancellationToken.None);

        // Assert
        await Assert.That(result).IsNull();
    }

    [Test]
    public async Task Should_ReturnCount_When_CountByUserSinceCalled()
    {
        // Arrange
        var client = new FakeCosmosRestClient();
        var repo = new CosmosNotificationRepository(client);
        var notification = new NotificationBuilder().Build();
        await repo.SaveAsync(notification, CancellationToken.None);

        // Act
        var since = new DateTimeOffset(2026, 1, 1, 0, 0, 0, TimeSpan.Zero);
        var result = await repo.CountByUserSinceAsync("user-1", since, CancellationToken.None);

        // Assert
        await Assert.That(result).IsGreaterThanOrEqualTo(1);
    }
}
