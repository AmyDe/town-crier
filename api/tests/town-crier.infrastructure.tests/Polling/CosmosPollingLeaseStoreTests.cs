using TownCrier.Infrastructure.Cosmos;
using TownCrier.Infrastructure.Polling;
using TownCrier.Infrastructure.Tests.Cosmos;

namespace TownCrier.Infrastructure.Tests.Polling;

public sealed class CosmosPollingLeaseStoreTests
{
    private static readonly DateTimeOffset Now = new(2026, 4, 23, 12, 0, 0, TimeSpan.Zero);

    [Test]
    public async Task TryAcquire_CreatesDocument_When_NoneExists()
    {
        var (store, _, _) = Build();

        var result = await store.TryAcquireAsync(TimeSpan.FromMinutes(5), CancellationToken.None);

        await Assert.That(result.Acquired).IsTrue();
        await Assert.That(result.Handle).IsNotNull();
        await Assert.That(result.Handle!.ETag).IsNotNull();
    }

    [Test]
    public async Task TryAcquire_ReturnsHeld_When_ExistingNotExpired()
    {
        var (store, _, _) = Build();
        var first = await store.TryAcquireAsync(TimeSpan.FromMinutes(5), CancellationToken.None);
        await Assert.That(first.Acquired).IsTrue();

        var second = await store.TryAcquireAsync(TimeSpan.FromMinutes(5), CancellationToken.None);

        await Assert.That(second.Acquired).IsFalse();
        await Assert.That(second.Held).IsTrue();
    }

    [Test]
    public async Task TryAcquire_ReacquiresViaCas_When_ExistingExpired()
    {
        var (store, _, time) = Build();
        var first = await store.TryAcquireAsync(TimeSpan.FromMinutes(5), CancellationToken.None);

        time.Advance(TimeSpan.FromMinutes(10)); // lease expired

        var second = await store.TryAcquireAsync(TimeSpan.FromMinutes(5), CancellationToken.None);

        await Assert.That(second.Acquired).IsTrue();
        await Assert.That(second.Handle!.ETag).IsNotEqualTo(first.Handle!.ETag);
    }

    [Test]
    public async Task Release_DeletesDocument_When_EtagMatches()
    {
        var (store, fake, _) = Build();
        var result = await store.TryAcquireAsync(TimeSpan.FromMinutes(5), CancellationToken.None);

        await store.ReleaseAsync(result.Handle!, CancellationToken.None);

        var readAfter = await fake.ReadDocumentWithETagAsync(
            CosmosContainerNames.Leases,
            "polling",
            "polling",
            CosmosJsonSerializerContext.Default.PollingLeaseDocument,
            CancellationToken.None);
        await Assert.That(readAfter.Document).IsNull();
    }

    [Test]
    public async Task Release_Swallows_When_EtagIsStale()
    {
        var (store, fake, _) = Build();
        var acquire = await store.TryAcquireAsync(TimeSpan.FromMinutes(5), CancellationToken.None);

        // Simulate a peer taking over via a direct replace.
        await fake.TryReplaceDocumentAsync(
            CosmosContainerNames.Leases,
            new PollingLeaseDocument
            {
                Id = "polling",
                HolderId = "peer",
                ExpiresAtUtc = Now.AddMinutes(5).ToString("o"),
            },
            "polling",
            acquire.Handle!.ETag,
            CosmosJsonSerializerContext.Default.PollingLeaseDocument,
            CancellationToken.None);

        // Release should not throw; fake records the attempt as PreconditionFailed.
        await store.ReleaseAsync(acquire.Handle, CancellationToken.None);
    }

    private static (CosmosPollingLeaseStore Store, FakeCosmosRestClient Fake, FakeTimeProvider Time) Build()
    {
        var fake = new FakeCosmosRestClient();
        var time = new FakeTimeProvider();
        time.SetUtcNow(Now);
        return (new CosmosPollingLeaseStore(fake, time), fake, time);
    }
}
