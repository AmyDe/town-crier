using System.Collections.Concurrent;
using TownCrier.Application.SavedApplications;
using TownCrier.Domain.SavedApplications;

namespace TownCrier.Infrastructure.SavedApplications;

public sealed class InMemorySavedApplicationRepository : ISavedApplicationRepository
{
    private readonly ConcurrentDictionary<string, SavedApplication> store = new();

    public Task SaveAsync(SavedApplication savedApplication, CancellationToken ct)
    {
        ArgumentNullException.ThrowIfNull(savedApplication);
        var key = MakeKey(savedApplication.UserId, savedApplication.ApplicationUid);
        this.store[key] = savedApplication;
        return Task.CompletedTask;
    }

    public Task DeleteAsync(string userId, string applicationUid, CancellationToken ct)
    {
        var key = MakeKey(userId, applicationUid);
        this.store.TryRemove(key, out _);
        return Task.CompletedTask;
    }

    public Task DeleteAllByUserIdAsync(string userId, CancellationToken ct)
    {
        ArgumentException.ThrowIfNullOrWhiteSpace(userId);

        var keysToRemove = this.store
            .Where(kvp => kvp.Value.UserId == userId)
            .Select(kvp => kvp.Key)
            .ToList();

        foreach (var key in keysToRemove)
        {
            this.store.TryRemove(key, out _);
        }

        return Task.CompletedTask;
    }

    public Task<IReadOnlyList<SavedApplication>> GetByUserIdAsync(string userId, CancellationToken ct)
    {
        var results = this.store.Values
            .Where(s => s.UserId == userId)
            .ToList();
        return Task.FromResult<IReadOnlyList<SavedApplication>>(results);
    }

    public Task<bool> ExistsAsync(string userId, string applicationUid, CancellationToken ct)
    {
        var key = MakeKey(userId, applicationUid);
        return Task.FromResult(this.store.ContainsKey(key));
    }

    public Task<IReadOnlyList<string>> GetUserIdsByApplicationUidAsync(string applicationUid, CancellationToken ct)
    {
        var userIds = this.store.Values
            .Where(s => s.ApplicationUid == applicationUid)
            .Select(s => s.UserId)
            .ToList();
        return Task.FromResult<IReadOnlyList<string>>(userIds);
    }

    private static string MakeKey(string userId, string applicationUid) => $"{userId}|{applicationUid}";
}
