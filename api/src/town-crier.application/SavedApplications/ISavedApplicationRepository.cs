using TownCrier.Domain.SavedApplications;

namespace TownCrier.Application.SavedApplications;

public interface ISavedApplicationRepository
{
    Task SaveAsync(SavedApplication savedApplication, CancellationToken ct);

    Task DeleteAsync(string userId, string applicationUid, CancellationToken ct);

    Task DeleteAllByUserIdAsync(string userId, CancellationToken ct);

    Task<IReadOnlyList<SavedApplication>> GetByUserIdAsync(string userId, CancellationToken ct);

    Task<bool> ExistsAsync(string userId, string applicationUid, CancellationToken ct);

    Task<IReadOnlyList<string>> GetUserIdsByApplicationUidAsync(string applicationUid, CancellationToken ct);
}
