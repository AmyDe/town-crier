using TownCrier.Application.Polling;

namespace TownCrier.Application.Tests.Polling;

public sealed class PollStateTests
{
    [Test]
    public async Task Should_ExposeLastPollTimeHighWaterMarkAndCursor_AsProperties()
    {
        // Arrange
        var lastPollTime = new DateTimeOffset(2026, 4, 19, 10, 0, 0, TimeSpan.Zero);
        var highWaterMark = new DateTimeOffset(2026, 4, 18, 8, 0, 0, TimeSpan.Zero);
        var cursor = new PollCursor(new DateOnly(2026, 4, 18), NextPage: 4, KnownTotal: 7200);

        // Act
        var state = new PollState(lastPollTime, highWaterMark, cursor);

        // Assert
        await Assert.That(state.LastPollTime).IsEqualTo(lastPollTime);
        await Assert.That(state.HighWaterMark).IsEqualTo(highWaterMark);
        await Assert.That(state.Cursor).IsEqualTo(cursor);
    }

    [Test]
    public async Task Should_AllowNullCursor_When_NoActiveResume()
    {
        // Arrange
        var lastPollTime = new DateTimeOffset(2026, 4, 19, 10, 0, 0, TimeSpan.Zero);
        var highWaterMark = new DateTimeOffset(2026, 4, 19, 10, 0, 0, TimeSpan.Zero);

        // Act
        var state = new PollState(lastPollTime, highWaterMark, Cursor: null);

        // Assert
        await Assert.That(state.Cursor).IsNull();
    }

    [Test]
    public async Task Should_AllowHighWaterMarkToLagLastPollTime_When_QuietAuthority()
    {
        // A quiet authority with no recent apps: LastPollTime tracks scheduling (now),
        // HighWaterMark tracks the PlanIt cursor (stays behind).
        var lastPollTime = new DateTimeOffset(2026, 4, 24, 12, 0, 0, TimeSpan.Zero);
        var highWaterMark = new DateTimeOffset(2026, 4, 18, 0, 0, 0, TimeSpan.Zero);

        var state = new PollState(lastPollTime, highWaterMark, Cursor: null);

        await Assert.That(state.HighWaterMark).IsLessThan(state.LastPollTime);
    }
}
