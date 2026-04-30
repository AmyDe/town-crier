using TownCrier.Domain.PlanningApplications;

namespace TownCrier.Application.Notifications;

/// <summary>
/// Port the polling pipeline uses to fan a transitioned application out to
/// the unified <see cref="DispatchDecisionEventCommandHandler"/>. The
/// infrastructure adapter wraps the handler; tests use an in-memory fake.
/// Mirrors the <see cref="DecisionAlerts.IDecisionAlertDispatcher"/> shape so
/// the polling handler can swap dispatchers cleanly while the alert pipeline
/// is migrated onto the unified decision-event flow.
/// </summary>
public interface IDecisionEventDispatcher
{
    Task DispatchAsync(PlanningApplication application, CancellationToken ct);
}
