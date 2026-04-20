using TownCrier.Domain.PlanningApplications;

namespace TownCrier.Application.PlanningApplications;

public interface IPlanningApplicationRepository
{
    Task UpsertAsync(PlanningApplication application, CancellationToken ct);

    Task<PlanningApplication?> GetByUidAsync(string uid, CancellationToken ct);

    // Partition-scoped lookup used on the poll-cycle hot path where the authority is
    // already known. Avoids a cross-partition fan-out by passing authorityCode as the
    // partition key. See bd tc-vidz.
    Task<PlanningApplication?> GetByUidAsync(string uid, string authorityCode, CancellationToken ct);

    Task<IReadOnlyCollection<PlanningApplication>> GetByAuthorityIdAsync(int authorityId, CancellationToken ct);

    Task<IReadOnlyCollection<PlanningApplication>> FindNearbyAsync(
        string authorityCode, double latitude, double longitude, double radiusMetres, CancellationToken ct);
}
