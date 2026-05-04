using TownCrier.Domain.NotificationState;
using TownCrier.Infrastructure.NotificationState;
using TownCrier.Infrastructure.Tests.Cosmos;

namespace TownCrier.Infrastructure.Tests.NotificationState;

public sealed class CosmosNotificationStateRepositoryTests
{
    private static readonly DateTimeOffset Now = new(2026, 5, 4, 10, 0, 0, TimeSpan.Zero);

    [Test]
    public async Task Should_ReturnNull_When_GetByUserIdForMissingUser()
    {
        // First-touch path: a user that has never had a notification-state document
        // returns null. The endpoint adapter (tc-1nsa.2) will Create + Save on this
        // miss to seed the watermark.
        var client = new FakeCosmosRestClient();
        var repo = new CosmosNotificationStateRepository(client);

        // Act
        var result = await repo.GetByUserIdAsync("auth0|user-missing", CancellationToken.None);

        // Assert
        await Assert.That(result).IsNull();
    }

    [Test]
    public async Task Should_RoundTripAggregate_When_SaveThenGetByUserId()
    {
        // Persistence boundary: the document mapping must hydrate UserId,
        // LastReadAt and Version exactly as written. Anything else and the
        // monotonic AdvanceTo guarantee rests on bad input.
        var client = new FakeCosmosRestClient();
        var repo = new CosmosNotificationStateRepository(client);
        var state = NotificationStateAggregate.Create("auth0|user-1", Now);
        state.MarkAllReadAt(Now.AddMinutes(5));

        // Act
        await repo.SaveAsync(state, CancellationToken.None);
        var hydrated = await repo.GetByUserIdAsync("auth0|user-1", CancellationToken.None);

        // Assert
        await Assert.That(hydrated).IsNotNull();
        await Assert.That(hydrated!.UserId).IsEqualTo("auth0|user-1");
        await Assert.That(hydrated.LastReadAt).IsEqualTo(Now.AddMinutes(5));
        await Assert.That(hydrated.Version).IsEqualTo(2);
    }

    [Test]
    public async Task Should_OverwriteState_When_SavedTwiceForSameUser()
    {
        // Upsert semantics: the second SaveAsync must replace the first. The
        // document is keyed on userId so concurrent advances would otherwise
        // leak old watermarks back over a newer one.
        var client = new FakeCosmosRestClient();
        var repo = new CosmosNotificationStateRepository(client);
        var initial = NotificationStateAggregate.Create("auth0|user-1", Now);
        await repo.SaveAsync(initial, CancellationToken.None);

        var advanced = NotificationStateAggregate.Reconstitute(
            "auth0|user-1", Now.AddDays(1), version: 5);

        // Act
        await repo.SaveAsync(advanced, CancellationToken.None);
        var hydrated = await repo.GetByUserIdAsync("auth0|user-1", CancellationToken.None);

        // Assert
        await Assert.That(hydrated).IsNotNull();
        await Assert.That(hydrated!.LastReadAt).IsEqualTo(Now.AddDays(1));
        await Assert.That(hydrated.Version).IsEqualTo(5);
    }

    [Test]
    public async Task Should_KeepStatesIsolatedByUser_When_TwoUsersSaved()
    {
        // Partitioning sanity: two users on the same container must not see
        // each other's watermarks. The container partitions on /userId, but
        // a regression in document mapping (e.g. a constant id) would silently
        // collapse them — this test guards against that.
        var client = new FakeCosmosRestClient();
        var repo = new CosmosNotificationStateRepository(client);
        await repo.SaveAsync(NotificationStateAggregate.Create("auth0|user-1", Now), CancellationToken.None);
        await repo.SaveAsync(NotificationStateAggregate.Create("auth0|user-2", Now.AddDays(1)), CancellationToken.None);

        // Act
        var alice = await repo.GetByUserIdAsync("auth0|user-1", CancellationToken.None);
        var bob = await repo.GetByUserIdAsync("auth0|user-2", CancellationToken.None);

        // Assert
        await Assert.That(alice).IsNotNull();
        await Assert.That(bob).IsNotNull();
        await Assert.That(alice!.LastReadAt).IsEqualTo(Now);
        await Assert.That(bob!.LastReadAt).IsEqualTo(Now.AddDays(1));
    }
}
