using System.Diagnostics.CodeAnalysis;
using TownCrier.Application.Polling;

namespace TownCrier.Infrastructure.Polling;

/// <summary>
/// Thread-safe <see cref="IPollJitter"/> backed by <see cref="Random.Shared"/>.
/// Returns a uniformly distributed offset in the range <c>[-bound, +bound]</c>.
/// </summary>
public sealed class SystemRandomPollJitter : IPollJitter
{
    [SuppressMessage(
        "Security",
        "CA5394:Do not use insecure randomness",
        Justification = "Jitter is non-security scheduling noise; no cryptographic guarantee needed.")]
    public TimeSpan NextOffset(TimeSpan bound)
    {
        if (bound <= TimeSpan.Zero)
        {
            return TimeSpan.Zero;
        }

        // NextInt64(minValue, maxValue) is exclusive on maxValue, so +1 to include +bound.Ticks.
        var ticks = Random.Shared.NextInt64(-bound.Ticks, bound.Ticks + 1);
        return TimeSpan.FromTicks(ticks);
    }
}
