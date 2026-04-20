using TownCrier.Domain.WatchZones;

namespace TownCrier.Application.WatchZones;

public interface IWatchZoneRepository
{
    Task SaveAsync(WatchZone zone, CancellationToken ct);

    Task<IReadOnlyCollection<WatchZone>> GetByUserIdAsync(string userId, CancellationToken ct);

    Task<WatchZone?> GetByUserAndZoneIdAsync(string userId, string zoneId, CancellationToken ct);

    Task DeleteAsync(string userId, string zoneId, CancellationToken ct);

    Task DeleteAllByUserIdAsync(string userId, CancellationToken ct);

    Task<IReadOnlyCollection<WatchZone>> FindZonesContainingAsync(
        double latitude, double longitude, CancellationToken ct);

    Task<IReadOnlyCollection<int>> GetDistinctAuthorityIdsAsync(CancellationToken ct);

    Task<Dictionary<int, int>> GetZoneCountsByAuthorityAsync(CancellationToken ct);
}
