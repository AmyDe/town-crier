using TownCrier.Application.Polling;

namespace TownCrier.Application.Tests.Polling;

public sealed class PollStateTests
{
    [Test]
    public async Task Should_ExposeLastPollTimeAndCursor_AsProperties()
    {
        // Arrange
        var lastPollTime = new DateTimeOffset(2026, 4, 19, 10, 0, 0, TimeSpan.Zero);
        var cursor = new PollCursor(new DateOnly(2026, 4, 18), NextPage: 4, KnownTotal: 7200);

        // Act
        var state = new PollState(lastPollTime, cursor);

        // Assert
        await Assert.That(state.LastPollTime).IsEqualTo(lastPollTime);
        await Assert.That(state.Cursor).IsEqualTo(cursor);
    }

    [Test]
    public async Task Should_AllowNullCursor_When_NoActiveResume()
    {
        // Arrange
        var lastPollTime = new DateTimeOffset(2026, 4, 19, 10, 0, 0, TimeSpan.Zero);

        // Act
        var state = new PollState(lastPollTime, Cursor: null);

        // Assert
        await Assert.That(state.Cursor).IsNull();
    }
}
