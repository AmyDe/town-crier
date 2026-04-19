namespace TownCrier.Application.Polling;

public sealed class CycleAlternatingAuthorityProvider : IActiveAuthorityProvider
{
    private readonly IWatchZoneActiveAuthorityProvider watchZoneProvider;
    private readonly IAllAuthorityIdProvider allAuthorityProvider;
    private readonly ICycleSelector cycleSelector;

    public CycleAlternatingAuthorityProvider(
        IWatchZoneActiveAuthorityProvider watchZoneProvider,
        IAllAuthorityIdProvider allAuthorityProvider,
        ICycleSelector cycleSelector)
    {
        this.watchZoneProvider = watchZoneProvider;
        this.allAuthorityProvider = allAuthorityProvider;
        this.cycleSelector = cycleSelector;
    }

    public Task<IReadOnlyCollection<int>> GetActiveAuthorityIdsAsync(CancellationToken ct)
    {
        return this.cycleSelector.GetCurrent() switch
        {
            CycleType.Seed => this.allAuthorityProvider.GetActiveAuthorityIdsAsync(ct),
            _ => this.watchZoneProvider.GetActiveAuthorityIdsAsync(ct),
        };
    }
}
