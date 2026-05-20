using TownCrier.Domain.PlanningApplications;

namespace TownCrier.Application.PlanningApplications;

public interface IPlanningApplicationRepository
{
    Task UpsertAsync(PlanningApplication application, CancellationToken ct);

    // Cross-partition fan-out — worker paths only (polling upsert, background lookup).
    // User-facing handlers must use GetByAuthorityAndNameAsync (point read, ~1 RU).
    Task<PlanningApplication?> GetByUidCrossPartitionAsync(string uid, CancellationToken ct);

    // Partition-scoped lookup used on the poll-cycle hot path where the authority is
    // already known. Avoids a cross-partition fan-out by passing authorityCode as the
    // partition key. See bd tc-vidz.
    Task<PlanningApplication?> GetByUidAsync(string uid, string authorityCode, CancellationToken ct);

    // Point read by document id (name) + partition key (authorityCode). ~1 RU.
    // Used by the user-facing GET /v1/applications/{authorityCode}/{**name} endpoint.
    // The Cosmos document id IS the application name (set in PlanningApplicationDocument.FromDomain).
    Task<PlanningApplication?> GetByAuthorityAndNameAsync(string authorityCode, string name, CancellationToken ct);

    Task<IReadOnlyCollection<PlanningApplication>> GetByAuthorityIdAsync(int authorityId, CancellationToken ct);

    Task<IReadOnlyCollection<PlanningApplication>> FindNearbyAsync(
        string authorityCode, double latitude, double longitude, double radiusMetres, CancellationToken ct);
}
