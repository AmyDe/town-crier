using TownCrier.Domain.WatchZones;

namespace TownCrier.Application.WatchZones;

public interface IWatchZoneRepository
{
    Task SaveAsync(WatchZone zone, CancellationToken ct);

    Task<IReadOnlyCollection<WatchZone>> GetByUserIdAsync(string userId, CancellationToken ct);

    Task<WatchZone?> GetByUserAndZoneIdAsync(string userId, string zoneId, CancellationToken ct);

    Task DeleteAsync(string userId, string zoneId, CancellationToken ct);

    Task DeleteAllByUserIdAsync(string userId, CancellationToken ct);

    // Cross-partition — worker paths only (polling: dispatch decision events and
    // find zones near a new application). Cost accepted for low-frequency background use.
    Task<IReadOnlyCollection<WatchZone>> FindZonesContainingCrossPartitionAsync(
        double latitude, double longitude, CancellationToken ct);

    // Cross-partition — worker path only (WatchZoneActiveAuthorityProvider for polling).
    Task<IReadOnlyCollection<int>> GetDistinctAuthorityIdsCrossPartitionAsync(CancellationToken ct);

    // Cross-partition — unused in production paths; kept for potential admin/reporting use.
    Task<Dictionary<int, int>> GetZoneCountsByAuthorityCrossPartitionAsync(CancellationToken ct);
}
