using TownCrier.Infrastructure.Polling;
using TownCrier.Infrastructure.Tests.Cosmos;

namespace TownCrier.Infrastructure.Tests.Polling;

public sealed class CosmosPollingLeaseStoreTests
{
    [Test]
    public async Task Should_Acquire_When_NoExistingLease()
    {
        var client = new FakeCosmosRestClient();
        var time = new FakeTimeProvider();
        time.SetUtcNow(new DateTimeOffset(2026, 4, 21, 12, 0, 0, TimeSpan.Zero));
        var store = new CosmosPollingLeaseStore(client, time);

        var acquired = await store.TryAcquireAsync(TimeSpan.FromMinutes(10), CancellationToken.None);

        await Assert.That(acquired).IsTrue();
    }

    [Test]
    public async Task Should_NotAcquire_When_ExistingLeaseIsStillLive()
    {
        var client = new FakeCosmosRestClient();
        var time = new FakeTimeProvider();
        time.SetUtcNow(new DateTimeOffset(2026, 4, 21, 12, 0, 0, TimeSpan.Zero));
        var storeA = new CosmosPollingLeaseStore(client, time);
        var storeB = new CosmosPollingLeaseStore(client, time);

        // A acquires first.
        var acquiredA = await storeA.TryAcquireAsync(TimeSpan.FromMinutes(10), CancellationToken.None);

        // Thirty seconds later, B attempts to acquire — A's lease TTL is still live.
        time.Advance(TimeSpan.FromSeconds(30));
        var acquiredB = await storeB.TryAcquireAsync(TimeSpan.FromMinutes(10), CancellationToken.None);

        await Assert.That(acquiredA).IsTrue();
        await Assert.That(acquiredB).IsFalse();
    }

    [Test]
    public async Task Should_Acquire_When_ExistingLeaseIsExpired()
    {
        var client = new FakeCosmosRestClient();
        var time = new FakeTimeProvider();
        time.SetUtcNow(new DateTimeOffset(2026, 4, 21, 12, 0, 0, TimeSpan.Zero));
        var storeA = new CosmosPollingLeaseStore(client, time);
        var storeB = new CosmosPollingLeaseStore(client, time);

        await storeA.TryAcquireAsync(TimeSpan.FromMinutes(10), CancellationToken.None);

        // Advance time past A's lease TTL — B can now take it.
        time.Advance(TimeSpan.FromMinutes(11));
        var acquiredB = await storeB.TryAcquireAsync(TimeSpan.FromMinutes(10), CancellationToken.None);

        await Assert.That(acquiredB).IsTrue();
    }

    [Test]
    public async Task Should_AllowSubsequentAcquire_After_Release()
    {
        var client = new FakeCosmosRestClient();
        var time = new FakeTimeProvider();
        time.SetUtcNow(new DateTimeOffset(2026, 4, 21, 12, 0, 0, TimeSpan.Zero));
        var storeA = new CosmosPollingLeaseStore(client, time);
        var storeB = new CosmosPollingLeaseStore(client, time);

        await storeA.TryAcquireAsync(TimeSpan.FromMinutes(10), CancellationToken.None);
        await storeA.ReleaseAsync(CancellationToken.None);

        // Still well within the original TTL — but the doc was deleted on release.
        time.Advance(TimeSpan.FromSeconds(5));
        var acquiredB = await storeB.TryAcquireAsync(TimeSpan.FromMinutes(10), CancellationToken.None);

        await Assert.That(acquiredB).IsTrue();
    }

    [Test]
    public async Task Should_NotThrow_When_ReleaseCalledWithoutAcquire()
    {
        var client = new FakeCosmosRestClient();
        var time = new FakeTimeProvider();
        time.SetUtcNow(new DateTimeOffset(2026, 4, 21, 12, 0, 0, TimeSpan.Zero));
        var store = new CosmosPollingLeaseStore(client, time);

        // Release is idempotent — safe to call from a finally block even if acquire failed.
        await store.ReleaseAsync(CancellationToken.None);
    }
}
