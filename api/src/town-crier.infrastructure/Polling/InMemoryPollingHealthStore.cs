using TownCrier.Application.Polling;
using TownCrier.Domain.Polling;

namespace TownCrier.Infrastructure.Polling;

public sealed class InMemoryPollingHealthStore : IPollingHealthStore
{
    private PollingHealth health = new();

    public Task<PollingHealth> GetAsync(CancellationToken ct)
    {
        return Task.FromResult(this.health);
    }

    public Task SaveAsync(PollingHealth health, CancellationToken ct)
    {
        this.health = health;
        return Task.CompletedTask;
    }
}
