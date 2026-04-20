using System.Collections.Concurrent;
using TownCrier.Application.DecisionAlerts;
using TownCrier.Domain.DecisionAlerts;

namespace TownCrier.Infrastructure.DecisionAlerts;

public sealed class InMemoryDecisionAlertRepository : IDecisionAlertRepository
{
    private readonly ConcurrentBag<DecisionAlert> store = [];

    public Task<DecisionAlert?> GetByUserAndApplicationAsync(string userId, string applicationUid, CancellationToken ct)
    {
        var alert = this.store.FirstOrDefault(
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
        var remaining = this.store.Where(a => a.UserId != userId).ToList();
        while (this.store.TryTake(out _))
        {
            // Drain the bag.
        }

        foreach (var alert in remaining)
        {
            this.store.Add(alert);
        }

        return Task.CompletedTask;
    }
}
