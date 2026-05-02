using TownCrier.Application.SavedApplications;
using TownCrier.Domain.SavedApplications;

namespace TownCrier.Application.Tests.PlanningApplications;

/// <summary>
/// Saved-application repository fake whose <see cref="SaveAsync"/> throws.
/// Lets tests prove that refresh-on-tap upsert failures never bubble out of the
/// detail handler — the read must still succeed (bd tc-udby).
/// </summary>
internal sealed class ThrowingOnSaveSavedApplicationRepository : ISavedApplicationRepository
{
    private readonly (string UserId, string ApplicationUid)? existsKey;

    public ThrowingOnSaveSavedApplicationRepository((string UserId, string ApplicationUid)? existsForUser = null)
    {
        this.existsKey = existsForUser;
    }

    public Task SaveAsync(SavedApplication savedApplication, CancellationToken ct) =>
        throw new InvalidOperationException("Simulated Cosmos write failure.");

    public Task DeleteAsync(string userId, string applicationUid, CancellationToken ct) => Task.CompletedTask;

    public Task DeleteAllByUserIdAsync(string userId, CancellationToken ct) => Task.CompletedTask;

    public Task<IReadOnlyList<SavedApplication>> GetByUserIdAsync(string userId, CancellationToken ct) =>
        Task.FromResult<IReadOnlyList<SavedApplication>>([]);

    public Task<bool> ExistsAsync(string userId, string applicationUid, CancellationToken ct)
    {
        if (this.existsKey is { } key
            && string.Equals(key.UserId, userId, StringComparison.Ordinal)
            && string.Equals(key.ApplicationUid, applicationUid, StringComparison.Ordinal))
        {
            return Task.FromResult(true);
        }

        return Task.FromResult(false);
    }

    public Task<IReadOnlyList<string>> GetUserIdsByApplicationUidAsync(string applicationUid, CancellationToken ct) =>
        Task.FromResult<IReadOnlyList<string>>([]);
}
