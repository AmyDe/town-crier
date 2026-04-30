using TownCrier.Application.Notifications;
using TownCrier.Domain.PlanningApplications;

namespace TownCrier.Infrastructure.Notifications;

/// <summary>
/// Adapter that wires the <see cref="IDecisionEventDispatcher"/> port used by
/// the polling pipeline to the in-process <see cref="DispatchDecisionEventCommandHandler"/>.
/// Mirrors the <c>DispatchDecisionAlertViaHandler</c> shape — keeps
/// <see cref="TownCrier.Application.Polling.PollPlanItCommandHandler"/> decoupled
/// from concrete handler types so the polling layer stays in Application.
/// </summary>
public sealed class DispatchDecisionEventViaHandler : IDecisionEventDispatcher
{
    private readonly DispatchDecisionEventCommandHandler handler;

    public DispatchDecisionEventViaHandler(DispatchDecisionEventCommandHandler handler)
    {
        this.handler = handler;
    }

    public async Task DispatchAsync(PlanningApplication application, CancellationToken ct)
    {
        var command = new DispatchDecisionEventCommand(application);
        await this.handler.HandleAsync(command, ct).ConfigureAwait(false);
    }
}
