using TownCrier.Domain.PlanningApplications;

namespace TownCrier.Application.PlanningApplications;

public interface IPlanningApplicationRepository
{
    Task UpsertAsync(PlanningApplication application, CancellationToken ct);

    Task<IReadOnlyCollection<PlanningApplication>> FindNearbyAsync(
        string authorityCode, double latitude, double longitude, double radiusMetres, CancellationToken ct);
}
