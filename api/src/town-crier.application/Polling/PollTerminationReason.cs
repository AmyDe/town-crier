namespace TownCrier.Application.Polling;

/// <summary>
/// How a poll cycle ended. Used by the worker to choose exit codes and by
/// the <c>towncrier.polling.cycles_completed</c> counter to tag cycle outcomes.
/// </summary>
public enum PollTerminationReason
{
    /// <summary>
    /// Every active authority was processed before the cycle ended.
    /// </summary>
    Natural = 0,

    /// <summary>
    /// The cycle was cancelled mid-loop by a bounded CancellationToken
    /// (typically the worker's replicaTimeout-driven CTS).
    /// </summary>
    TimeBounded = 1,

    /// <summary>
    /// The cycle stopped early because PlanIt returned HTTP 429.
    /// </summary>
    RateLimited = 2,
}
