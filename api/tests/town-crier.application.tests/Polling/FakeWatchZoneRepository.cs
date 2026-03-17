using TownCrier.Application.WatchZones;
using TownCrier.Domain.Geocoding;
using TownCrier.Domain.WatchZones;

namespace TownCrier.Application.Tests.Polling;

internal sealed class FakeWatchZoneRepository : IWatchZoneRepository
{
    private readonly List<WatchZone> zones = [];

    public void Add(WatchZone zone)
    {
        this.zones.Add(zone);
    }

    public Task SaveAsync(WatchZone zone, CancellationToken ct)
    {
        this.zones.RemoveAll(z => z.Id == zone.Id);
        this.zones.Add(zone);
        return Task.CompletedTask;
    }

    public void Remove(string zoneId)
    {
        this.zones.RemoveAll(z => z.Id == zoneId);
    }

    public Task<IReadOnlyCollection<int>> GetDistinctAuthorityIdsAsync(CancellationToken ct)
    {
        var authorityIds = this.zones
            .Select(z => z.AuthorityId)
            .Distinct()
            .ToList()
            .AsReadOnly();
        return Task.FromResult<IReadOnlyCollection<int>>(authorityIds);
    }

    public Task<IReadOnlyCollection<WatchZone>> FindZonesContainingAsync(
        double latitude, double longitude, CancellationToken ct)
    {
        var matching = this.zones
            .Where(z => DistanceMetres(z.Centre, new Coordinates(latitude, longitude)) <= z.RadiusMetres)
            .ToList();
        return Task.FromResult<IReadOnlyCollection<WatchZone>>(matching);
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
