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

    public Task SaveAsync(UserProfile profile, CancellationToken ct)
    {
        ArgumentNullException.ThrowIfNull(profile);
        this.store[profile.UserId] = profile;
        return Task.CompletedTask;
    }
}
