using System.Collections.Concurrent;
using TownCrier.Application.WatchZones;
using TownCrier.Domain.Geocoding;
using TownCrier.Domain.WatchZones;

namespace TownCrier.Infrastructure.WatchZones;

public sealed class InMemoryWatchZoneRepository : IWatchZoneRepository
{
    private readonly ConcurrentDictionary<string, WatchZone> zones = new();

    public Task SaveAsync(WatchZone zone, CancellationToken ct)
    {
        ArgumentNullException.ThrowIfNull(zone);
        this.zones[zone.Id] = zone;
        return Task.CompletedTask;
    }

    public Task<IReadOnlyCollection<WatchZone>> FindZonesContainingAsync(
        double latitude, double longitude, CancellationToken ct)
    {
        var point = new Coordinates(latitude, longitude);
        var matching = this.zones.Values
            .Where(z => DistanceMetres(z.Centre, point) <= z.RadiusMetres)
            .ToList();
        return Task.FromResult<IReadOnlyCollection<WatchZone>>(matching);
    }

    public Task<Dictionary<int, int>> GetZoneCountsByAuthorityAsync(CancellationToken ct)
    {
        var counts = this.zones.Values
            .GroupBy(z => z.AuthorityId)
            .ToDictionary(g => g.Key, g => g.Count());
        return Task.FromResult(counts);
    }

    public Task<IReadOnlyCollection<int>> GetDistinctAuthorityIdsAsync(CancellationToken ct)
    {
        var ids = this.zones.Values
            .Select(z => z.AuthorityId)
            .Distinct()
            .ToList();
        return Task.FromResult<IReadOnlyCollection<int>>(ids);
    }

    private static double DistanceMetres(Coordinates a, Coordinates b)
    {
        const double earthRadiusMetres = 6_371_000;
        var dLat = DegreesToRadians(b.Latitude - a.Latitude);
        var dLon = DegreesToRadians(b.Longitude - a.Longitude);
        var sinLat = Math.Sin(dLat / 2);
        var sinLon = Math.Sin(dLon / 2);
        var h = (sinLat * sinLat)
            + (Math.Cos(DegreesToRadians(a.Latitude)) * Math.Cos(DegreesToRadians(b.Latitude)) * sinLon * sinLon);
        return earthRadiusMetres * 2 * Math.Asin(Math.Sqrt(h));
    }

    private static double DegreesToRadians(double degrees) => degrees * Math.PI / 180;
}
