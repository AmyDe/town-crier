namespace TownCrier.Application.Polling;

/// <summary>
/// Runtime configuration for the PlanIt polling cycle. Bound from the
/// <c>Polling</c> configuration section in the worker host.
/// </summary>
public sealed record PollingOptions
{
    /// <summary>
    /// Gets the maximum number of PlanIt pages fetched per authority in a single
    /// poll cycle. <c>null</c> means unbounded (use natural end-of-data exit);
    /// any positive value voluntarily bails pagination so a backlogged authority
    /// cannot monopolise the cycle's rate budget before rotation advances.
    /// See <c>bd tc-l77h</c> for the seed-poll rationale.
    /// </summary>
    public int? MaxPagesPerAuthorityPerCycle { get; init; }

    /// <summary>
    /// Gets the TTL requested when the handler acquires the polling lease. Should
    /// cover a full cycle (default 10 minutes to match the worker's replicaTimeout).
    /// </summary>
    public TimeSpan LeaseTtl { get; init; } = TimeSpan.FromMinutes(10);

    /// <summary>
    /// Gets the soft wall-clock budget for a single handler invocation. When set,
    /// the handler checks at authority and page boundaries whether the deadline
    /// has elapsed and exits cleanly with <see cref="PollTerminationReason.TimeBounded"/>,
    /// saving a resumable cursor if mid-pagination. <c>null</c> disables the budget
    /// (handler only honours the outer CancellationToken). Sized to leave the
    /// orchestrator 60 s to publish-next + complete inside the 5-min Service Bus
    /// message lock. See <c>docs/specs/poll-handler-soft-budget.md</c>.
    /// </summary>
    public TimeSpan? HandlerBudget { get; init; }

    /// <summary>
    /// Gets the TTL requested when the orchestrator acquires a polling lease.
    /// </summary>
    public TimeSpan OrchestratorLeaseTtl { get; init; } = TimeSpan.FromMinutes(4.5);

    /// <summary>
    /// Gets the TTL requested when the bootstrap phase acquires a polling lease.
    /// </summary>
    public TimeSpan BootstrapLeaseTtl { get; init; } = TimeSpan.FromSeconds(60);

    /// <summary>
    /// Gets the delay before retrying a failed lease acquire operation.
    /// </summary>
    public TimeSpan LeaseAcquireRetryDelay { get; init; } = TimeSpan.FromSeconds(1);
}
