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

    public Task<IReadOnlyList<DecisionAlert>> GetByUserIdAsync(string userId, CancellationToken ct)
    {
        var alerts = this.store.Where(a => a.UserId == userId).ToList();
        return Task.FromResult<IReadOnlyList<DecisionAlert>>(alerts);
    }

    public Task SaveAsync(DecisionAlert alert, CancellationToken ct)
    {
        this.store.Add(alert);
        return Task.CompletedTask;
    }

    public Task DeleteAllByUserIdAsync(string userId, CancellationToken ct)
    {
        this.store.RemoveAll(a => a.UserId == userId);
        return Task.CompletedTask;
    }
}
