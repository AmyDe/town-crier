namespace TownCrier.Application.Polling;

/// <summary>
/// Tunables for the next-run scheduler. Defaults are the sibling bead's suggestions —
/// 5min natural cadence, 1min resume after a time-bounded cut-off, 30min cap on a
/// Retry-After header, 10s jitter bound.
/// </summary>
public sealed class PollNextRunSchedulerOptions
{
    public TimeSpan NaturalCadence { get; init; } = TimeSpan.FromMinutes(5);

    public TimeSpan TimeBoundedCadence { get; init; } = TimeSpan.FromMinutes(1);

    public TimeSpan RetryAfterCap { get; init; } = TimeSpan.FromMinutes(30);

    public TimeSpan RateLimitDefault { get; init; } = TimeSpan.FromMinutes(5);

    public TimeSpan JitterBound { get; init; } = TimeSpan.FromSeconds(10);
}
