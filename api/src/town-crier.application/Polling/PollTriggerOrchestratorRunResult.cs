namespace TownCrier.Application.Polling;

/// <summary>
/// Outcome of a single orchestrator run. <c>PollResult</c> is <c>null</c> only
/// when no trigger message was available at receive time.
/// </summary>
public sealed record PollTriggerOrchestratorRunResult(
    bool MessageReceived,
    bool PublishedNext,
    PollPlanItResult? PollResult);
