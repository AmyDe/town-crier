using TownCrier.Application.Polling;

namespace TownCrier.Application.Tests.Polling;

public sealed class RetryAfterParserTests
{
    [Test]
    public async Task Should_ReturnNull_When_HeaderIsNull()
    {
        var parsed = RetryAfterParser.Parse(null, DateTimeOffset.UtcNow);

        await Assert.That(parsed).IsNull();
    }

    [Test]
    public async Task Should_ReturnNull_When_HeaderIsEmpty()
    {
        var parsed = RetryAfterParser.Parse(string.Empty, DateTimeOffset.UtcNow);

        await Assert.That(parsed).IsNull();
    }

    [Test]
    public async Task Should_ReturnNull_When_HeaderIsMalformed()
    {
        var parsed = RetryAfterParser.Parse("not-a-number", DateTimeOffset.UtcNow);

        await Assert.That(parsed).IsNull();
    }

    [Test]
    public async Task Should_ReturnNull_When_HeaderIsNegative()
    {
        var parsed = RetryAfterParser.Parse("-5", DateTimeOffset.UtcNow);

        await Assert.That(parsed).IsNull();
    }

    [Test]
    public async Task Should_ReturnTimeSpan_When_HeaderIsDeltaSeconds()
    {
        var parsed = RetryAfterParser.Parse("120", DateTimeOffset.UtcNow);

        await Assert.That(parsed).IsEqualTo(TimeSpan.FromSeconds(120));
    }

    [Test]
    public async Task Should_ReturnZero_When_HeaderIsZeroSeconds()
    {
        var parsed = RetryAfterParser.Parse("0", DateTimeOffset.UtcNow);

        await Assert.That(parsed).IsEqualTo(TimeSpan.Zero);
    }

    [Test]
    public async Task Should_ReturnTimeSpan_When_HeaderIsHttpDateInFuture()
    {
        var now = new DateTimeOffset(2026, 4, 21, 12, 0, 0, TimeSpan.Zero);
        var future = now.AddSeconds(60).UtcDateTime.ToString("R", System.Globalization.CultureInfo.InvariantCulture);

        var parsed = RetryAfterParser.Parse(future, now);

        await Assert.That(parsed).IsEqualTo(TimeSpan.FromSeconds(60));
    }

    [Test]
    public async Task Should_ReturnZero_When_HeaderIsHttpDateInPast()
    {
        var now = new DateTimeOffset(2026, 4, 21, 12, 0, 0, TimeSpan.Zero);
        var past = now.AddSeconds(-60).UtcDateTime.ToString("R", System.Globalization.CultureInfo.InvariantCulture);

        var parsed = RetryAfterParser.Parse(past, now);

        await Assert.That(parsed).IsEqualTo(TimeSpan.Zero);
    }
}
