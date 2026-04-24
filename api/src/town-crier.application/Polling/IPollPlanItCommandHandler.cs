namespace TownCrier.Application.Polling;

/// <summary>
/// Port for the command handler that executes a single PlanIt poll cycle.
/// Extracted so <see cref="PollTriggerOrchestrator"/> can be tested without
/// wiring the full handler graph.
/// </summary>
public interface IPollPlanItCommandHandler
{
    /// <summary>
    /// Executes a single poll cycle and returns the result.
    /// </summary>
    /// <param name="command">The poll command.</param>
    /// <param name="ct">Cancellation token.</param>
    /// <returns>Result describing the outcome of the poll cycle.</returns>
    Task<PollPlanItResult> HandleAsync(PollPlanItCommand command, CancellationToken ct);
}
