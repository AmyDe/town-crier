using System.Collections.Concurrent;
using TownCrier.Application.PlanningApplications;
using TownCrier.Domain.PlanningApplications;

namespace TownCrier.Infrastructure.PlanningApplications;

public sealed class InMemoryPlanningApplicationRepository : IPlanningApplicationRepository
{
    private readonly ConcurrentDictionary<string, PlanningApplication> store = new();

    public Task UpsertAsync(PlanningApplication application, CancellationToken ct)
    {
        ArgumentNullException.ThrowIfNull(application);
        this.store[application.Name] = application;
        return Task.CompletedTask;
    }

    public Task<IReadOnlyCollection<PlanningApplication>> FindNearbyAsync(
        double latitude, double longitude, double radiusMetres, CancellationToken ct)
    {
        var nearby = this.store.Values
            .Where(a => a.Latitude.HasValue && a.Longitude.HasValue
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
