using TownCrier.Domain.PlanningApplications;

namespace TownCrier.Application.DecisionAlerts;

/// <summary>
/// Port the polling pipeline uses to fan a decided application out to the
/// decision-alert pipeline. The infrastructure adapter wraps
/// <see cref="DispatchDecisionAlertCommandHandler"/>; tests use an in-memory
/// fake. Mirrors the <c>INotificationEnqueuer</c> shape so the polling handler
/// stays decoupled from concrete handler types.
/// </summary>
public interface IDecisionAlertDispatcher
{
    Task DispatchAsync(PlanningApplication application, CancellationToken ct);
}
