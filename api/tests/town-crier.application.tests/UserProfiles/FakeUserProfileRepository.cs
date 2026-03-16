using TownCrier.Application.UserProfiles;
using TownCrier.Domain.UserProfiles;

namespace TownCrier.Application.Tests.UserProfiles;

internal sealed class FakeUserProfileRepository : IUserProfileRepository
{
    private readonly Dictionary<string, UserProfile> store = [];

    public int Count => this.store.Count;

    public Task<UserProfile?> GetByUserIdAsync(string userId, CancellationToken ct)
    {
        this.store.TryGetValue(userId, out var profile);
        return Task.FromResult(profile);
    }

    public Task SaveAsync(UserProfile profile, CancellationToken ct)
    {
        this.store[profile.UserId] = profile;
        return Task.CompletedTask;
    }

    public UserProfile? GetByUserId(string userId)
    {
        this.store.TryGetValue(userId, out var profile);
        return profile;
    }
}
