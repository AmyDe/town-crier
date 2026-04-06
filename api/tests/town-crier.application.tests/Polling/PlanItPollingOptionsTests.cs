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
}
