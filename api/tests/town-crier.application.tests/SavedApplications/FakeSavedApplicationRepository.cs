using TownCrier.Application.SavedApplications;
using TownCrier.Domain.SavedApplications;

namespace TownCrier.Application.Tests.SavedApplications;

internal sealed class FakeSavedApplicationRepository : ISavedApplicationRepository
{
    private readonly List<SavedApplication> store = [];

    public int Count => this.store.Count;

    public Task SaveAsync(SavedApplication savedApplication, CancellationToken ct)
    {
        var existing = this.store.FindIndex(
            s => s.UserId == savedApplication.UserId && s.ApplicationUid == savedApplication.ApplicationUid);

        if (existing >= 0)
        {
            this.store[existing] = savedApplication;
        }
        else
        {
            this.store.Add(savedApplication);
        }

        return Task.CompletedTask;
    }

    public Task DeleteAsync(string userId, string applicationUid, CancellationToken ct)
    {
        this.store.RemoveAll(s => s.UserId == userId && s.ApplicationUid == applicationUid);
        return Task.CompletedTask;
    }

    public Task<IReadOnlyList<SavedApplication>> GetByUserIdAsync(string userId, CancellationToken ct)
    {
        var results = this.store.Where(s => s.UserId == userId).ToList();
        return Task.FromResult<IReadOnlyList<SavedApplication>>(results);
    }

    public Task<bool> ExistsAsync(string userId, string applicationUid, CancellationToken ct)
    {
        var exists = this.store.Any(s => s.UserId == userId && s.ApplicationUid == applicationUid);
        return Task.FromResult(exists);
    }
}
