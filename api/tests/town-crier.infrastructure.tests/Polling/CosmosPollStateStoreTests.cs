using TownCrier.Application.Polling;
using TownCrier.Infrastructure.Polling;
using TownCrier.Infrastructure.Tests.Cosmos;

namespace TownCrier.Infrastructure.Tests.Polling;

public sealed class CosmosPollStateStoreTests
{
    [Test]
    public async Task Should_RoundTripState_WithoutCursor()
    {
        // Arrange
        var client = new FakeCosmosRestClient();
        var store = new CosmosPollStateStore(client);
        var lastPollTime = new DateTimeOffset(2026, 4, 18, 12, 0, 0, TimeSpan.Zero);

        // Act
        await store.SaveAsync(authorityId: 45, lastPollTime, cursor: null, CancellationToken.None);
        var state = await store.GetAsync(authorityId: 45, CancellationToken.None);

        // Assert
        await Assert.That(state).IsNotNull();
        await Assert.That(state!.LastPollTime).IsEqualTo(lastPollTime);
        await Assert.That(state.Cursor).IsNull();
    }

    [Test]
    public async Task Should_RoundTripState_WithCursor()
    {
        // Arrange
        var client = new FakeCosmosRestClient();
        var store = new CosmosPollStateStore(client);
        var lastPollTime = new DateTimeOffset(2026, 4, 18, 12, 0, 0, TimeSpan.Zero);
        var cursor = new PollCursor(
            DifferentStart: new DateOnly(2026, 4, 18),
            NextPage: 4,
            KnownTotal: 7200);

        // Act
        await store.SaveAsync(authorityId: 123, lastPollTime, cursor, CancellationToken.None);
        var state = await store.GetAsync(authorityId: 123, CancellationToken.None);

        // Assert
        await Assert.That(state).IsNotNull();
        await Assert.That(state!.LastPollTime).IsEqualTo(lastPollTime);
        await Assert.That(state.Cursor).IsNotNull();
        await Assert.That(state.Cursor!.DifferentStart).IsEqualTo(cursor.DifferentStart);
        await Assert.That(state.Cursor.NextPage).IsEqualTo(cursor.NextPage);
        await Assert.That(state.Cursor.KnownTotal).IsEqualTo(cursor.KnownTotal);
    }

    [Test]
    public async Task Should_ReturnNull_When_NoStateExistsForAuthority()
    {
        // Arrange
        var client = new FakeCosmosRestClient();
        var store = new CosmosPollStateStore(client);

        // Act
        var state = await store.GetAsync(authorityId: 999, CancellationToken.None);

        // Assert
        await Assert.That(state).IsNull();
    }

    [Test]
    public async Task Should_ClearCursor_When_SavedWithNullCursorAfterActiveCursor()
    {
        // Arrange — saving a null cursor on top of an existing cursor must clear it.
        var client = new FakeCosmosRestClient();
        var store = new CosmosPollStateStore(client);
        var lastPollTime = new DateTimeOffset(2026, 4, 18, 12, 0, 0, TimeSpan.Zero);
        var cursor = new PollCursor(new DateOnly(2026, 4, 18), NextPage: 4, KnownTotal: 7200);
        await store.SaveAsync(authorityId: 77, lastPollTime, cursor, CancellationToken.None);

        // Act
        var advanced = new DateTimeOffset(2026, 4, 19, 12, 0, 0, TimeSpan.Zero);
        await store.SaveAsync(authorityId: 77, advanced, cursor: null, CancellationToken.None);
        var state = await store.GetAsync(authorityId: 77, CancellationToken.None);

        // Assert
        await Assert.That(state).IsNotNull();
        await Assert.That(state!.LastPollTime).IsEqualTo(advanced);
        await Assert.That(state.Cursor).IsNull();
    }
}
