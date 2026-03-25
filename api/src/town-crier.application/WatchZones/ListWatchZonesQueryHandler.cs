namespace TownCrier.Application.WatchZones;

public sealed class ListWatchZonesQueryHandler
{
    private readonly IWatchZoneRepository watchZoneRepository;

    public ListWatchZonesQueryHandler(IWatchZoneRepository watchZoneRepository)
    {
        this.watchZoneRepository = watchZoneRepository;
    }

    public async Task<ListWatchZonesResult> HandleAsync(ListWatchZonesQuery query, CancellationToken ct)
    {
        ArgumentNullException.ThrowIfNull(query);

        var zones = await this.watchZoneRepository.GetByUserIdAsync(query.UserId, ct).ConfigureAwait(false);

        var summaries = zones
            .Select(z => new WatchZoneSummary(
                z.Id,
                z.Name,
                z.Centre.Latitude,
                z.Centre.Longitude,
                z.RadiusMetres,
                z.AuthorityId))
            .ToList();

        return new ListWatchZonesResult(summaries);
    }
}
