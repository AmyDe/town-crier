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

// Removed: LeaseHeld (was = 3)
// Reason: The orchestrator now exits with LeaseUnavailable flag instead of returning
// a termination reason. LeaseHeld was a signal for the handler to publish a termination
// message — serialisation now happens in the orchestrator itself, so poll results only
// flow from handlers that actually ran. See ADR 0024 amendment and feat/polling-lease-cas.
