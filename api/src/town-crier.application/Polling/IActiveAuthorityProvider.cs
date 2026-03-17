namespace TownCrier.Application.Polling;

public interface IActiveAuthorityProvider
{
    Task<IReadOnlyCollection<int>> GetActiveAuthorityIdsAsync(CancellationToken ct);
}
