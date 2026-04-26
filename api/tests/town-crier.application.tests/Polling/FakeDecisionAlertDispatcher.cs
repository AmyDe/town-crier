using TownCrier.Application.DecisionAlerts;
using TownCrier.Domain.PlanningApplications;

namespace TownCrier.Application.Tests.Polling;

internal sealed class FakeDecisionAlertDispatcher : IDecisionAlertDispatcher
{
    private readonly List<PlanningApplication> dispatched = [];

    public IReadOnlyList<PlanningApplication> Dispatched => this.dispatched;

    public Task DispatchAsync(PlanningApplication application, CancellationToken ct)
    {
        this.dispatched.Add(application);
        return Task.CompletedTask;
    }
}
