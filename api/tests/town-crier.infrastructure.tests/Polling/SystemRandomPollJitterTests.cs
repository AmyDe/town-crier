using TownCrier.Application.Polling;
using TownCrier.Infrastructure.Polling;

namespace TownCrier.Infrastructure.Tests.Polling;

public sealed class SystemRandomPollJitterTests
{
    [Test]
    public async Task Should_ReturnOffsetWithinBound_When_NextOffsetCalled()
    {
        var jitter = new SystemRandomPollJitter();
        var bound = TimeSpan.FromSeconds(10);

        for (var i = 0; i < 200; i++)
        {
            var offset = jitter.NextOffset(bound);
            await Assert.That(offset).IsGreaterThanOrEqualTo(-bound);
            await Assert.That(offset).IsLessThanOrEqualTo(bound);
        }
    }

    [Test]
    public async Task Should_ReturnZero_When_BoundIsZero()
    {
        var jitter = new SystemRandomPollJitter();

        var offset = jitter.NextOffset(TimeSpan.Zero);

        await Assert.That(offset).IsEqualTo(TimeSpan.Zero);
    }

    [Test]
    public async Task Should_ProduceVariedOffsets_When_CalledRepeatedly()
    {
        var jitter = new SystemRandomPollJitter();
        var bound = TimeSpan.FromSeconds(30);

        var offsets = new HashSet<long>();
        for (var i = 0; i < 50; i++)
        {
            offsets.Add(jitter.NextOffset(bound).Ticks);
        }

        // Expect at least a handful of distinct values — confirms it isn't always zero
        // or a fixed constant (thread-safe Random.Shared is non-deterministic).
        await Assert.That(offsets.Count).IsGreaterThan(5);
    }
}
