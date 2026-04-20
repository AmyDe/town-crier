using TownCrier.Application.PlanningApplications;
using TownCrier.Domain.PlanningApplications;

namespace TownCrier.Application.Tests.Polling;

internal sealed class FakePlanningApplicationRepository : IPlanningApplicationRepository
{
    private readonly Dictionary<string, PlanningApplication> store = [];

    public int UpsertCallCount { get; private set; }

    public int GetByUidWithAuthorityCallCount { get; private set; }

    public int GetByUidWithoutAuthorityCallCount { get; private set; }

    public IReadOnlyCollection<PlanningApplication> GetAll() => this.store.Values.ToList();

    public PlanningApplication? GetByName(string name)
    {
        this.store.TryGetValue(name, out var app);
        return app;
    }

    public Task UpsertAsync(PlanningApplication application, CancellationToken ct)
    {
        this.UpsertCallCount++;
        this.store[application.Name] = application;
        return Task.CompletedTask;
    }

    public Task<PlanningApplication?> GetByUidAsync(string uid, CancellationToken ct)
    {
        this.GetByUidWithoutAuthorityCallCount++;
        var app = this.store.Values.FirstOrDefault(a => a.Uid == uid);
        return Task.FromResult(app);
    }

    public Task<PlanningApplication?> GetByUidAsync(string uid, string authorityCode, CancellationToken ct)
    {
        this.GetByUidWithAuthorityCallCount++;
        var app = this.store.Values.FirstOrDefault(
            a => a.Uid == uid
                && a.AreaId.ToString(System.Globalization.CultureInfo.InvariantCulture) == authorityCode);
        return Task.FromResult(app);
    }

    public Task<IReadOnlyCollection<PlanningApplication>> GetByAuthorityIdAsync(int authorityId, CancellationToken ct)
    {
        var apps = this.store.Values.Where(a => a.AreaId == authorityId).ToList();
        return Task.FromResult<IReadOnlyCollection<PlanningApplication>>(apps);
    }

    public Task<IReadOnlyCollection<PlanningApplication>> FindNearbyAsync(
        string authorityCode, double latitude, double longitude, double radiusMetres, CancellationToken ct)
    {
        var nearby = this.store.Values
            .Where(a => a.AreaId.ToString(System.Globalization.CultureInfo.InvariantCulture) == authorityCode
                && a.Latitude.HasValue && a.Longitude.HasValue
                && DistanceMetres(latitude, longitude, a.Latitude.Value, a.Longitude.Value) <= radiusMetres)
            .ToList();
        return Task.FromResult<IReadOnlyCollection<PlanningApplication>>(nearby);
    }

    private static double DistanceMetres(double lat1, double lon1, double lat2, double lon2)
    {
        const double earthRadiusMetres = 6_371_000;
        var dLat = DegreesToRadians(lat2 - lat1);
        var dLon = DegreesToRadians(lon2 - lon1);
        var sinLat = Math.Sin(dLat / 2);
        var sinLon = Math.Sin(dLon / 2);
        var h = (sinLat * sinLat)
            + (Math.Cos(DegreesToRadians(lat1)) * Math.Cos(DegreesToRadians(lat2)) * sinLon * sinLon);
        return earthRadiusMetres * 2 * Math.Asin(Math.Sqrt(h));
    }

    private static double DegreesToRadians(double degrees) => degrees * Math.PI / 180;
}
