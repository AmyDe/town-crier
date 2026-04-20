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
}
