namespace TownCrier.Application.Polling;

/// <summary>
/// Decides when the next poll run should be scheduled, given how the previous run
/// ended. See sibling bead tc-8uoj for the policy rationale.
/// </summary>
public sealed class PollNextRunScheduler
{
    private readonly PollNextRunSchedulerOptions options;
    private readonly IPollJitter jitter;

    public PollNextRunScheduler(PollNextRunSchedulerOptions options, IPollJitter jitter)
    {
        this.options = options;
        this.jitter = jitter;
    }

    public DateTimeOffset ComputeNextRun(
        PollTerminationReason reason,
        TimeSpan? retryAfter,
        DateTimeOffset now)
    {
        return reason switch
        {
            PollTerminationReason.RateLimited => now + this.ComputeRateLimitedDelay(retryAfter),
            PollTerminationReason.TimeBounded => now + this.options.TimeBoundedCadence,
            _ => now + this.options.NaturalCadence,
        };
    }

    private TimeSpan ComputeRateLimitedDelay(TimeSpan? retryAfter)
    {
        TimeSpan baseDelay;
        if (retryAfter is { } suggested)
        {
            baseDelay = suggested > this.options.RetryAfterCap ? this.options.RetryAfterCap : suggested;
        }
        else
        {
            baseDelay = this.options.RateLimitDefault;
        }

        return baseDelay + this.jitter.NextOffset(this.options.JitterBound);
    }
}
