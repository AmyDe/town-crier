using TownCrier.Domain.SavedApplications;

namespace TownCrier.Application.SavedApplications;

public interface ISavedApplicationRepository
{
    Task SaveAsync(SavedApplication savedApplication, CancellationToken ct);

    Task DeleteAsync(string userId, string applicationUid, CancellationToken ct);

    Task DeleteAllByUserIdAsync(string userId, CancellationToken ct);

    Task<IReadOnlyList<SavedApplication>> GetByUserIdAsync(string userId, CancellationToken ct);

    Task<bool> ExistsAsync(string userId, string applicationUid, CancellationToken ct);

    /// <summary>
    /// Returns the userIds who have saved the given application — scoped to a
    /// specific council. PlanIt uids are only unique within a council, so
    /// cross-authority matches would dispatch the wrong council's payload
    /// (bd tc-th98 / GH#384).
    /// </summary>
    /// <param name="applicationUid">The PlanIt-assigned uid for the application.</param>
    /// <param name="authorityId">The PlanIt areaId for the council that issued the uid.</param>
    /// <param name="ct">Cancellation token.</param>
    /// <returns>The userIds who have saved the given application within the specified authority.</returns>
    Task<IReadOnlyList<string>> GetUserIdsForApplicationAsync(string applicationUid, int authorityId, CancellationToken ct);
}
