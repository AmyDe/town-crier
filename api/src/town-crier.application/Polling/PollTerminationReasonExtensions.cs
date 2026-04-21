namespace TownCrier.Application.Polling;

public static class PollTerminationReasonExtensions
{
    /// <summary>
    /// Maps a <see cref="PollTerminationReason"/> to the lowercase, snake-case string
    /// used as the <c>termination</c> tag on the
    /// <c>towncrier.polling.cycles_completed</c> counter.
    /// </summary>
    /// <param name="reason">The termination reason to convert.</param>
    /// <returns>The telemetry tag value.</returns>
    /// <exception cref="ArgumentOutOfRangeException">
    /// Thrown when a new <see cref="PollTerminationReason"/> member is added without
    /// updating this mapping. Fail-fast prevents silent telemetry misattribution.
    /// </exception>
    public static string ToTelemetryValue(this PollTerminationReason reason) => reason switch
    {
        PollTerminationReason.Natural => "natural",
        PollTerminationReason.TimeBounded => "time_bounded",
        PollTerminationReason.RateLimited => "rate_limited",
        PollTerminationReason.LeaseHeld => "lease_held",
        _ => throw new ArgumentOutOfRangeException(
            nameof(reason),
            reason,
            $"Unknown {nameof(PollTerminationReason)} value '{reason}'. Update {nameof(PollTerminationReasonExtensions)}.{nameof(ToTelemetryValue)} when adding new reasons."),
    };
}
