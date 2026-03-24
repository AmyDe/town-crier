using TownCrier.Domain.PlanningApplications;

namespace TownCrier.Application.PlanningApplications;

public interface IPlanningApplicationRepository
{
    Task UpsertAsync(PlanningApplication application, CancellationToken ct);

    Task<PlanningApplication?> GetByUidAsync(string uid, CancellationToken ct);

    Task<IReadOnlyCollection<PlanningApplication>> GetByAuthorityIdAsync(int authorityId, CancellationToken ct);

    Task<IReadOnlyCollection<PlanningApplication>> FindNearbyAsync(
        string authorityCode, double latitude, double longitude, double radiusMetres, CancellationToken ct);
}
