using TownCrier.Application.DecisionAlerts;
using TownCrier.Domain.DecisionAlerts;

namespace TownCrier.Application.Tests.DecisionAlerts;

internal sealed class FakeDecisionAlertRepository : IDecisionAlertRepository
{
    private readonly List<DecisionAlert> store = [];

    public IReadOnlyList<DecisionAlert> All => this.store;

    public Task<DecisionAlert?> GetByUserAndApplicationAsync(string userId, string applicationUid, CancellationToken ct)
    {
        var alert = this.store.Find(
            a => a.UserId == userId && a.ApplicationUid == applicationUid);
        return Task.FromResult(alert);
    }

    public Task SaveAsync(DecisionAlert alert, CancellationToken ct)
    {
        this.store.Add(alert);
        return Task.CompletedTask;
    }
}
