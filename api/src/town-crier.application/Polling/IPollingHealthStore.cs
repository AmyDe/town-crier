using TownCrier.Domain.Polling;

namespace TownCrier.Application.Polling;

public interface IPollingHealthStore
{
    Task<PollingHealth> GetAsync(CancellationToken ct);

    Task SaveAsync(PollingHealth health, CancellationToken ct);
}
