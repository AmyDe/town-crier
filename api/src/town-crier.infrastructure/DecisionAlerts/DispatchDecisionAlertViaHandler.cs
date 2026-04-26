using TownCrier.Application.DecisionAlerts;
using TownCrier.Domain.PlanningApplications;

namespace TownCrier.Infrastructure.DecisionAlerts;

/// <summary>
/// Adapter that wires the <see cref="IDecisionAlertDispatcher"/> port used by
/// the polling pipeline to the in-process <see cref="DispatchDecisionAlertCommandHandler"/>.
/// Mirrors the <c>DispatchNotificationEnqueuer</c> shape — keeps
/// <see cref="TownCrier.Application.Polling.PollPlanItCommandHandler"/> decoupled
/// from concrete handler types so the polling layer stays in Application.
/// </summary>
public sealed class DispatchDecisionAlertViaHandler : IDecisionAlertDispatcher
{
    private readonly DispatchDecisionAlertCommandHandler handler;

    public DispatchDecisionAlertViaHandler(DispatchDecisionAlertCommandHandler handler)
    {
        this.handler = handler;
    }

    public async Task DispatchAsync(PlanningApplication application, CancellationToken ct)
    {
        var command = new DispatchDecisionAlertCommand(application);
        await this.handler.HandleAsync(command, ct).ConfigureAwait(false);
    }
}
