using TownCrier.Application.Polling;

namespace TownCrier.Application.Tests.Polling;

public sealed class PlanItPollingOptionsTests
{
    [Test]
    public async Task Should_UseDefaultCooldown_When_NoOptionsProvided()
    {
        var options = new PlanItPollingOptions();

        await Assert.That(options.RateLimitCooldownSeconds).IsEqualTo(30);
        await Assert.That(options.RateLimitCooldown).IsEqualTo(TimeSpan.FromSeconds(30));
    }

    [Test]
    public async Task Should_ComputeCooldownFromSeconds_When_SecondsPropertySet()
    {
        var options = new PlanItPollingOptions { RateLimitCooldownSeconds = 15.5 };

        await Assert.That(options.RateLimitCooldown).IsEqualTo(TimeSpan.FromSeconds(15.5));
    }

    [Test]
    public async Task Should_ReturnZeroCooldown_When_SecondsSetToZero()
    {
        var options = new PlanItPollingOptions { RateLimitCooldownSeconds = 0 };

        await Assert.That(options.RateLimitCooldown).IsEqualTo(TimeSpan.Zero);
    }
}
