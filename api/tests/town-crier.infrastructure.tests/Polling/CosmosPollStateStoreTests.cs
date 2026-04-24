using System.Text.Json;
using TownCrier.Application.Polling;
using TownCrier.Infrastructure.Cosmos;
using TownCrier.Infrastructure.Polling;
using TownCrier.Infrastructure.Tests.Cosmos;

namespace TownCrier.Infrastructure.Tests.Polling;

public sealed class CosmosPollStateStoreTests
{
    private static readonly int[] QuietAndBusyAuthorities = [156, 200];

    [Test]
    public async Task Should_RoundTripState_WithoutCursor()
    {
        // Arrange
        var client = new FakeCosmosRestClient();
        var store = new CosmosPollStateStore(client);
        var lastPollTime = new DateTimeOffset(2026, 4, 18, 12, 0, 0, TimeSpan.Zero);
        var highWaterMark = new DateTimeOffset(2026, 4, 17, 10, 0, 0, TimeSpan.Zero);

        // Act
        await store.SaveAsync(authorityId: 45, lastPollTime, highWaterMark, cursor: null, CancellationToken.None);
        var state = await store.GetAsync(authorityId: 45, CancellationToken.None);

        // Assert
        await Assert.That(state).IsNotNull();
        await Assert.That(state!.LastPollTime).IsEqualTo(lastPollTime);
        await Assert.That(state.HighWaterMark).IsEqualTo(highWaterMark);
        await Assert.That(state.Cursor).IsNull();
    }

    [Test]
    public async Task Should_RoundTripState_WithCursor()
    {
        // Arrange
        var client = new FakeCosmosRestClient();
        var store = new CosmosPollStateStore(client);
        var lastPollTime = new DateTimeOffset(2026, 4, 18, 12, 0, 0, TimeSpan.Zero);
        var highWaterMark = new DateTimeOffset(2026, 4, 18, 8, 0, 0, TimeSpan.Zero);
        var cursor = new PollCursor(
            DifferentStart: new DateOnly(2026, 4, 18),
            NextPage: 4,
            KnownTotal: 7200);

        // Act
        await store.SaveAsync(authorityId: 123, lastPollTime, highWaterMark, cursor, CancellationToken.None);
        var state = await store.GetAsync(authorityId: 123, CancellationToken.None);

        // Assert
        await Assert.That(state).IsNotNull();
        await Assert.That(state!.LastPollTime).IsEqualTo(lastPollTime);
        await Assert.That(state.HighWaterMark).IsEqualTo(highWaterMark);
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
        var highWaterMark = new DateTimeOffset(2026, 4, 18, 10, 0, 0, TimeSpan.Zero);
        var cursor = new PollCursor(new DateOnly(2026, 4, 18), NextPage: 4, KnownTotal: 7200);
        await store.SaveAsync(authorityId: 77, lastPollTime, highWaterMark, cursor, CancellationToken.None);

        // Act
        var advanced = new DateTimeOffset(2026, 4, 19, 12, 0, 0, TimeSpan.Zero);
        var advancedHighWaterMark = new DateTimeOffset(2026, 4, 19, 8, 0, 0, TimeSpan.Zero);
        await store.SaveAsync(authorityId: 77, advanced, advancedHighWaterMark, cursor: null, CancellationToken.None);
        var state = await store.GetAsync(authorityId: 77, CancellationToken.None);

        // Assert
        await Assert.That(state).IsNotNull();
        await Assert.That(state!.LastPollTime).IsEqualTo(advanced);
        await Assert.That(state.HighWaterMark).IsEqualTo(advancedHighWaterMark);
        await Assert.That(state.Cursor).IsNull();
    }

    [Test]
    public async Task Should_FallBackToLastPollTime_When_LegacyDocumentHasNoHighWaterMark()
    {
        // Backward compat: legacy documents written before tc-m6fx stored only
        // LastPollTime (which doubled as the PlanIt cursor). When reading such a
        // document, HighWaterMark should fall back to LastPollTime so cursor behaviour
        // is preserved until the next write populates both fields.
        var client = new FakeCosmosRestClient();
        var legacyJson =
            """
            {
                "id": "poll-state-42",
                "lastPollTime": "2026-04-18T12:00:00.0000000+00:00",
                "authorityId": 42
            }
            """;
        client.SeedDocument(
            CosmosContainerNames.PollState,
            documentId: "poll-state-42",
            partitionKey: "poll-state-42",
            json: legacyJson);
        var store = new CosmosPollStateStore(client);

        // Act
        var state = await store.GetAsync(authorityId: 42, CancellationToken.None);

        // Assert
        await Assert.That(state).IsNotNull();
        var expected = new DateTimeOffset(2026, 4, 18, 12, 0, 0, TimeSpan.Zero);
        await Assert.That(state!.LastPollTime).IsEqualTo(expected);
        await Assert.That(state.HighWaterMark).IsEqualTo(expected);
        await Assert.That(state.Cursor).IsNull();
    }

    [Test]
    public async Task Should_SortByLastPollTime_NotHighWaterMark_When_QuietAuthorityHasStaleHwm()
    {
        // Core regression guard for tc-m6fx: the scheduling order must use
        // LastPollTime so a quiet authority (stale HighWaterMark, fresh
        // LastPollTime) drops to the back of the queue immediately after being
        // polled — rather than being re-selected as "oldest" every cycle.
        var client = new FakeCosmosRestClient();
        var store = new CosmosPollStateStore(client);

        // Authority A: polled one minute ago but HWM is 7 days stale (quiet LPA).
        var now = new DateTimeOffset(2026, 4, 24, 12, 0, 0, TimeSpan.Zero);
        var quietRecentPoll = now.AddMinutes(-1);
        var quietStaleHwm = now.AddDays(-7);
        await store.SaveAsync(
            authorityId: 156, quietRecentPoll, quietStaleHwm, cursor: null, CancellationToken.None);

        // Authority B: polled one hour ago, HWM fresh.
        var busyLastPoll = now.AddHours(-1);
        var busyHwm = now.AddHours(-1);
        await store.SaveAsync(
            authorityId: 200, busyLastPoll, busyHwm, cursor: null, CancellationToken.None);

        // Act
        var order = await store.GetLeastRecentlyPolledAsync(QuietAndBusyAuthorities, CancellationToken.None);

        // Assert — B sorts before A despite A having the older HWM.
        await Assert.That(order[0]).IsEqualTo(200);
        await Assert.That(order[1]).IsEqualTo(156);
    }
}
