using TownCrier.Infrastructure.Subscriptions;
using TownCrier.Infrastructure.Tests.Cosmos;

namespace TownCrier.Infrastructure.Tests.Subscriptions;

public sealed class CosmosNotificationIdempotencyStoreTests
{
    private const string NotificationUuid = "9ad9eb1c-1f0b-4e21-9d2e-2f1f8c8e0001";

    [Test]
    public async Task Should_ReturnFalse_When_NotificationHasNotBeenProcessed()
    {
        var client = new FakeCosmosRestClient();
        var store = new CosmosNotificationIdempotencyStore(client);

        var processed = await store.IsProcessedAsync(NotificationUuid, CancellationToken.None);

        await Assert.That(processed).IsFalse();
    }

    [Test]
    public async Task Should_ReturnTrue_When_NotificationHasBeenMarkedProcessed()
    {
        var client = new FakeCosmosRestClient();
        var store = new CosmosNotificationIdempotencyStore(client);

        await store.MarkProcessedAsync(NotificationUuid, CancellationToken.None);
        var processed = await store.IsProcessedAsync(NotificationUuid, CancellationToken.None);

        await Assert.That(processed).IsTrue();
    }

    [Test]
    public async Task Should_NotThrow_When_SameNotificationMarkedProcessedTwice()
    {
        var client = new FakeCosmosRestClient();
        var store = new CosmosNotificationIdempotencyStore(client);

        await store.MarkProcessedAsync(NotificationUuid, CancellationToken.None);
        await store.MarkProcessedAsync(NotificationUuid, CancellationToken.None);

        var processed = await store.IsProcessedAsync(NotificationUuid, CancellationToken.None);
        await Assert.That(processed).IsTrue();
    }

    [Test]
    public async Task Should_TrackEachNotificationIndependently()
    {
        var client = new FakeCosmosRestClient();
        var store = new CosmosNotificationIdempotencyStore(client);

        await store.MarkProcessedAsync(NotificationUuid, CancellationToken.None);

        var otherProcessed = await store.IsProcessedAsync(
            "9ad9eb1c-1f0b-4e21-9d2e-2f1f8c8e0002", CancellationToken.None);
        await Assert.That(otherProcessed).IsFalse();
    }
}
