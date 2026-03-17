using System.Collections.Concurrent;
using TownCrier.Application.UserProfiles;
using TownCrier.Domain.UserProfiles;

namespace TownCrier.Infrastructure.UserProfiles;

public sealed class InMemoryUserProfileRepository : IUserProfileRepository
{
    private readonly ConcurrentDictionary<string, UserProfile> store = new();

    public Task<UserProfile?> GetByUserIdAsync(string userId, CancellationToken ct)
    {
        this.store.TryGetValue(userId, out var profile);
        return Task.FromResult(profile);
    }

    public Task<IReadOnlyList<UserProfile>> GetAllByTierAsync(SubscriptionTier tier, CancellationToken ct)
    {
        var profiles = this.store.Values
            .Where(p => p.Tier == tier)
            .ToList();
        return Task.FromResult<IReadOnlyList<UserProfile>>(profiles);
    }

    public Task<UserProfile?> GetByOriginalTransactionIdAsync(string originalTransactionId, CancellationToken ct)
    {
        var profile = this.store.Values
            .FirstOrDefault(p => p.OriginalTransactionId == originalTransactionId);
        return Task.FromResult(profile);
    }

    public Task SaveAsync(UserProfile profile, CancellationToken ct)
    {
        ArgumentNullException.ThrowIfNull(profile);
        this.store[profile.UserId] = profile;
        return Task.CompletedTask;
    }

    public Task DeleteAsync(string userId, CancellationToken ct)
    {
        this.store.TryRemove(userId, out _);
        return Task.CompletedTask;
    }
}
