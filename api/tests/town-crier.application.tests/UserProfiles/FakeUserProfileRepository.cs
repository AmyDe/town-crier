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

    public Task<UserProfile?> GetByEmailCrossPartitionAsync(string email, CancellationToken ct)
    {
        var profile = this.store.Values
            .FirstOrDefault(p => string.Equals(p.Email, email, StringComparison.OrdinalIgnoreCase));
        return Task.FromResult(profile);
    }

    public Task<IReadOnlyList<UserProfile>> GetAllByTierCrossPartitionAsync(SubscriptionTier tier, CancellationToken ct)
    {
        var profiles = this.store.Values
            .Where(p => p.Tier == tier)
            .ToList();
        return Task.FromResult<IReadOnlyList<UserProfile>>(profiles);
    }

    public Task<IReadOnlyList<UserProfile>> GetAllByDigestDayCrossPartitionAsync(DayOfWeek digestDay, CancellationToken ct)
    {
        var profiles = this.store.Values
            .Where(p => p.NotificationPreferences.DigestDay == digestDay)
            .ToList();
        return Task.FromResult<IReadOnlyList<UserProfile>>(profiles);
    }

    public Task<UserProfile?> GetByOriginalTransactionIdCrossPartitionAsync(string originalTransactionId, CancellationToken ct)
    {
        var profile = this.store.Values
            .FirstOrDefault(p => p.OriginalTransactionId == originalTransactionId);
        return Task.FromResult(profile);
    }

    public Task SaveAsync(UserProfile profile, CancellationToken ct)
    {
        this.store[profile.UserId] = profile;
        return Task.CompletedTask;
    }

    public Task DeleteAsync(string userId, CancellationToken ct)
    {
        this.store.Remove(userId);
        return Task.CompletedTask;
    }

    public Task<IReadOnlyList<UserProfile>> GetDormantCrossPartitionAsync(DateTimeOffset cutoff, CancellationToken ct)
    {
        var profiles = this.store.Values
            .Where(p => p.LastActiveAt < cutoff)
            .ToList();
        return Task.FromResult<IReadOnlyList<UserProfile>>(profiles);
    }

    public Task<UserProfilePage> ListCrossPartitionAsync(
        string? emailSearch, int pageSize, string? continuationToken, CancellationToken ct)
    {
        var profiles = this.store.Values.AsEnumerable();

        if (emailSearch is not null)
        {
            profiles = profiles.Where(p =>
                p.Email is not null &&
                p.Email.Contains(emailSearch, StringComparison.OrdinalIgnoreCase));
        }

        var result = profiles.OrderBy(p => p.Email, StringComparer.OrdinalIgnoreCase).ToList();
        return Task.FromResult(new UserProfilePage(result, null));
    }

    public UserProfile? GetByUserId(string userId)
    {
        this.store.TryGetValue(userId, out var profile);
        return profile;
    }
}
