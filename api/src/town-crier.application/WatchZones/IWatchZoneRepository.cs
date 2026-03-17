using TownCrier.Domain.WatchZones;

namespace TownCrier.Application.WatchZones;

public interface IWatchZoneRepository
{
    Task<IReadOnlyCollection<WatchZone>> FindZonesContainingAsync(
        double latitude, double longitude, CancellationToken ct);

    Task<IReadOnlyCollection<int>> GetDistinctAuthorityIdsAsync(CancellationToken ct);
}
