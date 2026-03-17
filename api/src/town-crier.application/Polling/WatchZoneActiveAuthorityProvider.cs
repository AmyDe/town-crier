using TownCrier.Application.WatchZones;

namespace TownCrier.Application.Polling;

public sealed class WatchZoneActiveAuthorityProvider : IActiveAuthorityProvider
{
    private readonly IWatchZoneRepository watchZoneRepository;

    public WatchZoneActiveAuthorityProvider(IWatchZoneRepository watchZoneRepository)
    {
        this.watchZoneRepository = watchZoneRepository;
    }

    public async Task<IReadOnlyCollection<int>> GetActiveAuthorityIdsAsync(CancellationToken ct)
    {
        return await this.watchZoneRepository.GetDistinctAuthorityIdsAsync(ct).ConfigureAwait(false);
    }
}
